package main

import (
 "context"
 "crypto/rand"
 "crypto/subtle"
 "embed"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "io/fs"
 "log"
 "net"
 "net/http"
 "net/url"
 "html"
 "os"
 "os/exec"
 "path/filepath"
 "regexp"
 "strconv"
 "strings"
 "sync"
 "time"
)
//go:embed web/*
var files embed.FS
var client=&http.Client{Timeout:8*time.Second}
type session struct{Expires time.Time; CSRF string}
var sessions=map[string]session{}
var mu sync.Mutex
var modemMu sync.Mutex
type attempt struct{Count int;Since time.Time}; var attempts=map[string]attempt{}
var subMu sync.Mutex
var subscription=map[string]any{"available":false,"message":"尚未读取网络承载参数"}
var subAt time.Time
var subBusy bool
var demo=os.Getenv("CONSOLE_DEMO")=="1"
var pages=map[string]string{"imei":"Advanced_MIMEI_Content.asp","network":"Advanced_Settings_Content.asp","bands":"Advanced_Settings_Band.asp","wifi2":"Advanced_Wireless2g_Content.asp","wifi5":"Advanced_Wireless_Content.asp","lan":"Advanced_LAN_Content.asp","firewall":"Advanced_BasicFirewall_Content.asp","ports":"Advanced_VirtualServer_Content.asp","system":"Advanced_System_Content.asp","sleep":"Advanced_System_AutoSleep.asp","sms":"smsInbox.asp","smsout":"smsOutbox.asp","smssend":"smsSend.asp","logs":"Main_LogStatus_Content.asp"}
func cmd(name string,args ...string)(string,error){ctx,cancel:=context.WithTimeout(context.Background(),7*time.Second);defer cancel();b,e:=exec.CommandContext(ctx,name,args...).CombinedOutput();if ctx.Err()!=nil{return "",errors.New("设备响应超时")};return strings.TrimSpace(string(b)),e}
func nv(k string)string{s,_:=cmd("nvram","get",k);return s}
func token()string{b:=make([]byte,32);if _,e:=rand.Read(b);e!=nil{panic(e)};return hex.EncodeToString(b)}
func out(w http.ResponseWriter,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");json.NewEncoder(w).Encode(v)}
func fail(w http.ResponseWriter,n int,s string){w.WriteHeader(n);out(w,map[string]any{"error":s})}
func body(r *http.Request,v any)error{return json.NewDecoder(io.LimitReader(r.Body,65536)).Decode(v)}
func native(path string)(string,error){res,e:=client.Get("http://127.0.0.1/"+path);if e!=nil{return "",e};defer res.Body.Close();if res.StatusCode!=200{return "",fmt.Errorf("固件接口 HTTP %d",res.StatusCode)};b,e:=io.ReadAll(io.LimitReader(res.Body,2<<20));return string(b),e}
var keyRE=regexp.MustCompile(`([{,]\s*)([A-Za-z_][A-Za-z0-9_]*)\s*:`)
var hexRE=regexp.MustCompile(`0x[0-9a-fA-F]+`)
func parseStatus(s string)(map[string]any,error){a:=strings.Index(s,"{");b:=strings.LastIndex(s,"}");if a<0||b<a{return nil,errors.New("状态格式错误")};s=s[a:b+1];s=keyRE.ReplaceAllString(s,`${1}"${2}":`);s=hexRE.ReplaceAllStringFunc(s,func(v string)string{n,_:=strconv.ParseUint(v[2:],16,64);return strconv.FormatUint(n,10)});var v map[string]any;e:=json.Unmarshal([]byte(s),&v);return v,e}
func status()(map[string]any,error){if demo{return map[string]any{"native":map[string]any{"network_type":"NR5G-NSA","rsrp":-99,"rsrp_5g":-106,"bat_value":4,"bat_charge":2,"simcard_status":1,"uptime":map[string]int{"days":1,"hours":3,"minutes":26},"ram":map[string]int{"total":227888,"used":112640,"free":115248},"cpu":map[string]int{"busy":103,"total":1000}},"operator":"中国移动","band":"N78","model":"MF650","firmware":"MF650_V2.3","temp":43.2,"demo":true,"traffic":map[string]int64{"rx":234567890,"tx":23456789}},nil};s,e:=native("system_status_data.asp");if e!=nil{return nil,e};v,e:=parseStatus(s);if e!=nil{return nil,e};m:=map[string]any{"native":v,"operator":nv("operator"),"band":nv("band"),"model":"MF650 / SDX55","firmware":nv("fw_ver"),"time":time.Now().Unix(),"cellid":nv("cellid"),"pci":nv("pci"),"earfcn":nv("earfcn"),"apn":nv("modem_apn")};b,_:=os.ReadFile("/proc/net/dev");var rx,tx int64;for _,line:=range strings.Split(string(b),"\n"){p:=strings.SplitN(line,":",2);if len(p)!=2{continue};n:=strings.TrimSpace(p[0]);if !strings.HasPrefix(n,"rmnet_data")&&!strings.HasPrefix(n,"rmnet0"){continue};f:=strings.Fields(p[1]);if len(f)>=9{r,_:=strconv.ParseInt(f[0],10,64);t,_:=strconv.ParseInt(f[8],10,64);rx+=r;tx+=t}};m["traffic"]=map[string]int64{"rx":rx,"tx":tx};if sample,e:=readBattery();e==nil{m["battery_live"]=sample;batteryMu.Lock();m["charging_paused"]=batteryCfg.Paused;batteryMu.Unlock()};subMu.Lock();if apn,ok:=subscription["apn"].(string);ok&&apn!=""{m["apn"]=apn};subMu.Unlock();if t,ok:=cpuTemperature();ok{m["temp"]=t};return m,nil}
var nameRE=regexp.MustCompile(`(?i)\bname\s*=\s*["']([a-zA-Z][a-zA-Z0-9_]*)["']`)
func handler(w http.ResponseWriter,r *http.Request){w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("X-Frame-Options","DENY");w.Header().Set("Referrer-Policy","same-origin");if r.URL.Path=="/healthz"{out(w,map[string]string{"status":"ok","version":"2.0"});return}
 if r.URL.Path=="/api/login"{if r.Method!="POST"{fail(w,405,"请登录");return};var b struct{Username string;Password string};if body(r,&b)!=nil{fail(w,400,"请求无效");return};host,_,_:=net.SplitHostPort(r.RemoteAddr);mu.Lock();a:=attempts[host];if time.Since(a.Since)>time.Minute*5{a=attempt{Since:time.Now()}};a.Count++;attempts[host]=a;mu.Unlock();if a.Count>12{fail(w,429,"尝试过多，请 5 分钟后重试");return};pw:=nv("http_passwd");user:=nv("http_username");if demo{pw="demo";user="admin"};if user==""||pw==""||subtle.ConstantTimeCompare([]byte(strings.TrimSpace(b.Username)),[]byte(user))!=1||subtle.ConstantTimeCompare([]byte(b.Password),[]byte(pw))!=1{time.Sleep(700*time.Millisecond);fail(w,401,"用户名或密码不正确");return};id,csrf:=token(),token();mu.Lock();sessions[id]=session{time.Now().Add(2*time.Hour),csrf};for k,v:=range sessions{if time.Now().After(v.Expires){delete(sessions,k)}};persistSessions();mu.Unlock();http.SetCookie(w,&http.Cookie{Name:"console_session",Value:id,Path:"/",HttpOnly:true,SameSite:http.SameSiteStrictMode,MaxAge:7200});out(w,map[string]string{"csrf":csrf});return}
 if !strings.HasPrefix(r.URL.Path,"/api/"){sub,_:=fs.Sub(files,"web");http.FileServer(http.FS(sub)).ServeHTTP(w,r);return}
 c,e:=r.Cookie("console_session");var ss session;if e==nil{mu.Lock();ss=sessions[c.Value];mu.Unlock()};if time.Now().After(ss.Expires){fail(w,401,"请使用设备管理员密码登录");return};if r.Method!="GET"{if r.Header.Get("X-CSRF-Token")!=ss.CSRF{fail(w,403,"会话校验失败，请刷新页面");return};if origin:=r.Header.Get("Origin");origin!=""&&origin!="http://"+r.Host{fail(w,403,"请求来源不匹配");return};if demo{fail(w,403,"演示模式不修改设备");return}}
 switch r.URL.Path{
 case "/api/battery", "/api/battery/settings", "/api/battery/control":batteryAPI(w,r)
 case "/api/session":out(w,map[string]any{"csrf":ss.CSRF,"demo":demo})
 case "/api/logout":mu.Lock();delete(sessions,c.Value);persistSessions();mu.Unlock();http.SetCookie(w,&http.Cookie{Name:"console_session",Value:"",Path:"/",MaxAge:-1,HttpOnly:true});out(w,map[string]bool{"ok":true})
 case "/api/sms":
 p:="sms_in.asp";if r.URL.Query().Get("box")=="out"{p="sms_out.asp"};s,e:=native(p);if e!=nil{fail(w,502,"读取短信失败");return};start,end:=strings.Index(s,"["),strings.LastIndex(s,"]");if start<0||end<start{fail(w,502,"短信数据格式错误");return};var messages []any;if e=json.Unmarshal([]byte(s[start:end+1]),&messages);e!=nil{fail(w,502,"固件短信格式暂不支持");return};out(w,map[string]any{"messages":messages})
 case "/api/status":v,e:=status();if e!=nil{fail(w,502,"无法读取固件状态："+e.Error());return};out(w,v)
 case "/api/page":p,ok:=pages[r.URL.Query().Get("id")];if !ok{fail(w,404,"页面不存在");return};s,e:=native(p);if e!=nil{fail(w,502,"读取设置失败");return};values:=map[string]string{};if r.URL.Query().Get("id")=="network"{values["modem_apn"]=nv("sim1_apn")};out(w,map[string]any{"html":s,"path":p,"values":values})
 case "/api/save":if r.Method!="POST"{fail(w,405,"请使用保存按钮");return};var b struct{Page string;Fields map[string]string};if body(r,&b)!=nil{fail(w,400,"数据格式错误");return};if (b.Page=="wifi2"||b.Page=="wifi5")&&wifiUnifyManaged(){fail(w,409,"请先关闭双频合一，再分别修改无线配置。");return};path,ok:=pages[b.Page];if !ok||b.Page=="logs"||strings.HasPrefix(b.Page,"sms"){fail(w,400,"该页面不能使用通用保存");return};html,e:=native(path);if e!=nil{fail(w,502,"无法校验原始表单");return};allowed:=map[string]bool{};for _,m:=range nameRE.FindAllStringSubmatch(html,-1){allowed[m[1]]=true};v:=url.Values{};for k,x:=range b.Fields{if allowed[k]&&len(x)<4096{v.Set(k,x)}};mode:=v.Get("action_mode");if !validMode(b.Page,mode){fail(w,400,"保存动作不支持");return};for _,key:=range []string{"action_script","sid_list","group_id"}{v.Set(key,hiddenValue(html,key))};if b.Page=="network"{if mode==" ApplyApn "{v.Set("sys_settings_mode","")}else{v.Set("sys_settings_mode","OPERATIONMODE")}}
 if b.Page=="imei"&&!regexp.MustCompile(`^[0-9]{15}$`).MatchString(v.Get("imei")){fail(w,400,"IMEI 必须为 15 位数字");return};if b.Page=="bands"{prefix:="band";if mode==" Apply5G "{prefix="band_n"};selected:=false;for key,x:=range b.Fields{if strings.HasPrefix(key,prefix)&&(mode!=" Apply4G "||!strings.HasPrefix(key,"band_n"))&&strings.HasSuffix(key,"_enable_x")&&x=="1"{selected=true}};if !selected{fail(w,400,"至少选择一个目标网络的频段");return}}
 if b.Page=="wifi2"||b.Page=="wifi5"{prefix:="rt_";if b.Page=="wifi5"{prefix="wl_"};ssid:=v.Get(prefix+"ssid");if len([]byte(ssid))<1||len([]byte(ssid))>32{fail(w,400,"Wi-Fi 名称需要 1–32 字节");return};pass:=v.Get(prefix+"wpa_psk");if pass!=""&&(len(pass)<8||len(pass)>63){fail(w,400,"Wi-Fi 密码需要 8–63 位");return}}
 if b.Page=="system"{if v.Get("http_username")==""{v.Del("http_username")};if v.Get("http_passwd2")==""{v.Del("http_passwd")}};if b.Page=="system"&&v.Get("http_passwd2")!=""{pw:=v.Get("http_passwd2");if pw!=v.Get("v_password2")||len(pw)<8||len(pw)>32||!regexp.MustCompile(`[0-9]`).MatchString(pw)||!regexp.MustCompile(`[a-zA-Z]`).MatchString(pw)||!regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(pw){fail(w,400,"两次密码应一致，长度 8–32 位，并包含字母、数字和特殊字符");return};v.Set("http_passwd",pw)};if b.Page=="system"&&v.Get("http_username")==""&&v.Get("http_passwd")==""{fail(w,400,"请填写要修改的账号或密码");return}
 v.Set("current_page",path);v.Set("next_page","");v.Set("next_host","");req,_:=http.NewRequest("POST","http://127.0.0.1/start_apply.htm",strings.NewReader(v.Encode()));req.Header.Set("Content-Type","application/x-www-form-urlencoded");req.Header.Set("Referer","http://127.0.0.1/"+path);res,e:=client.Do(req);if e!=nil{fail(w,502,"提交连接中断，请重新读取设备设置确认");return};defer res.Body.Close();if res.StatusCode!=200{fail(w,502,"固件拒绝保存");return};reply,_:=io.ReadAll(io.LimitReader(res.Body,128<<10));if strings.Contains(string(reply),"alert(")||strings.Contains(strings.ToLower(string(reply)),"invalid"){fail(w,422,"固件返回校验提示，请检查填写内容");return};if b.Page=="system"&&(v.Get("http_passwd")!=""||v.Get("http_username")!=""){mu.Lock();sessions=map[string]session{};persistSessions();mu.Unlock()};out(w,map[string]any{"ok":true,"message":"已提交到固件，请重新读取设置确认是否生效。"})
 case "/api/power":
 if r.Method!="POST"{fail(w,405,"请使用操作按钮");return};var b struct{Action string};if body(r,&b)!=nil{fail(w,400,"请求无效");return};program:="";switch b.Action{case "reboot":program="reboot";case "shutdown":program="poweroff";default:fail(w,400,"不支持的电源操作");return};path,e:=exec.LookPath(program);if e!=nil{fail(w,501,"固件没有提供该电源操作");return};out(w,map[string]any{"ok":true,"message":"设备将在 2 秒后执行操作，当前连接会断开。"});go func(){time.Sleep(2*time.Second);if e:=exec.Command(path).Run();e!=nil{log.Printf("Power action failed: %v",e)}}()
 case "/api/subscription":
 subMu.Lock();if !subBusy&&(subAt.IsZero()||time.Since(subAt)>time.Minute||r.URL.Query().Get("refresh")=="1"){subBusy=true;go readSubscription()};snapshot:=map[string]any{};for k,v:=range subscription{snapshot[k]=v};snapshot["updating"]=subBusy;snapshot["updated_at"]=subAt.Unix();subMu.Unlock();out(w,snapshot)
 case "/api/imei":
 value:=nv("imei");if !regexp.MustCompile(`^[0-9]{15}$`).MatchString(value){raw,e:=cmd("ubus","call","qmi_svr","get_imei","{}");if e==nil{value=regexp.MustCompile(`[0-9]{15}`).FindString(raw)}};out(w,map[string]string{"imei":value})
 case "/api/memory/clean":memoryCleanAPI(w,r)
 case "/api/wifi/unify":wifiUnifyAPI(w,r)
 case "/api/sim":simAPI(w,r)
 case "/api/cell", "/api/cell/lock", "/api/cell/unlock":cellAPI(w,r)
 case "/api/at":if r.Method!="POST"{fail(w,405,"方法不支持");return};var b struct{Command string};if body(r,&b)!=nil{fail(w,400,"请求无效");return};if !validAT(b.Command){fail(w,400,"请输入以 AT 开头的单行指令，最多 256 字符");return};modemMu.Lock();s,e:=cmd("/usr/bin/atcmd",b.Command);modemMu.Unlock();if e!=nil{fail(w,502,"指令未完成或设备超时");return};out(w,map[string]string{"response":s})
 default:fail(w,404,"接口不存在")
 }
}
func parseQoS(raw string)map[string]any{result:=map[string]any{"available":false,"source":"AT+CGEQOSRDP","response_state":"empty","message":"当前查询未带回 AMBR / QCI 数据；不代表 SIM 卡没有签约速率，也不代表模块故障。"};for _,line:=range strings.Split(raw,"\n"){line=strings.TrimSpace(line);if !strings.HasPrefix(line,"+CGEQOSRDP:"){continue};fields:=strings.Split(strings.TrimSpace(strings.TrimPrefix(line,"+CGEQOSRDP:")),",");if len(fields)<2{continue};n:=func(i int)float64{if i>=len(fields){return 0};v,_:=strconv.ParseFloat(strings.TrimSpace(fields[i]),64);return v};qci:=n(1);if qci<1{continue};result["available"]=true;result["response_state"]="available";result["cid"]=n(0);result["qci"]=qci;result["message"]="当前数据承载返回的 AMBR / QCI 参数";if len(fields)>=8{if v:=n(6);v>0{result["down_kbps"]=v};if v:=n(7);v>0{result["up_kbps"]=v}};if len(fields)>=6{if v:=n(4);v>0{result["max_down_kbps"]=v};if v:=n(5);v>0{result["max_up_kbps"]=v}};break};return result}
func activeContext(raw string)(int,string){m:=regexp.MustCompile(`(?m)\+CGCONTRDP:\s*([0-9]+),\s*[0-9]+,\s*"?([^,"\r\n ]+)`).FindStringSubmatch(raw);if len(m)!=3{return 0,""};cid,_:=strconv.Atoi(m[1]);return cid,m[2]}
func readSubscription(){if captured,ok:=currentDiagSubscription();ok{subMu.Lock();subscription=captured;subAt=time.Now();subBusy=false;subMu.Unlock();return};modemMu.Lock();contextRaw,_:=cmd("/usr/bin/atcmd","AT+CGCONTRDP");cid,apn:=activeContext(contextRaw);command:="AT+CGEQOSRDP";if cid>0{command+="="+strconv.Itoa(cid)};raw,e:=cmd("/usr/bin/atcmd",command);modemMu.Unlock();result:=parseQoS(raw);result["source"]=command;if cid>0{result["cid"]=cid};if apn!=""{result["apn"]=apn};if e!=nil{result["response_state"]="failed";result["message"]="本次参数查询未完成，请稍后重试。"}else if strings.Contains(raw,"ERROR"){result["response_state"]="rejected";result["message"]="当前固件拒绝此参数查询，暂未取得 AMBR / QCI 数据。"}else if result["available"]==false{result["message"]="已按当前活动 CID 查询，设备只回复 OK，未带回 AMBR / QCI。数据连接仍正常，不能用空响应判断硬件不支持。"};subMu.Lock();subscription=result;subAt=time.Now();subBusy=false;subMu.Unlock()}

func validAT(s string)bool{return len(s)>=2&&len(s)<=256&&strings.HasPrefix(strings.ToUpper(s),"AT")&&!strings.ContainsAny(s,"\r\n\x00")}
func cpuTemperature()(float64,bool){paths,_:=filepath.Glob("/sys/class/thermal/thermal_zone*/type");for _,p:=range paths{b,e:=os.ReadFile(p);if e!=nil||strings.TrimSpace(string(b))!="cpu0-0-usr"{continue};b,e=os.ReadFile(filepath.Join(filepath.Dir(p),"temp"));if e!=nil{continue};f,e:=strconv.ParseFloat(strings.TrimSpace(string(b)),64);if e!=nil{continue};f/=1000;if f>0&&f<130{return f,true}};return 0,false}
func persistSessions(){b,e:=json.Marshal(sessions);if e==nil{_ = os.WriteFile("/data/airlink/sessions.json",b,0600)}}
func hiddenValue(s,key string)string{tags:=regexp.MustCompile(`(?i)<input\b[^>]*>`).FindAllString(s,-1);for _,tag:=range tags{nm:=nameRE.FindStringSubmatch(tag);if len(nm)<2||nm[1]!=key{continue};m:=regexp.MustCompile(`(?i)\bvalue\s*=\s*["']([^"']*)["']`).FindStringSubmatch(tag);if len(m)>1{return html.UnescapeString(m[1])}};return ""}
func validMode(page,mode string)bool{switch page{case "imei":return mode==" ApplyIMEI ";case "bands":return mode==" Apply4G "||mode==" Apply5G ";case "network":return mode==" Apply "||mode==" ApplyApn ";case "lan":return mode==" Restart ";case "sleep":return mode==" ApplyAutoSleep ";default:return mode==" Apply "}}
func main(){if len(os.Args)==3&&os.Args[1]=="--import-subscription"{if e:=importDiagSubscription(os.Args[2]);e!=nil{log.Fatal(e)};return};if len(os.Args)>1&&os.Args[1]=="--battery-check"{loadBatteryConfig();sample,e:=readBattery();outJSON,_:=json.Marshal(map[string]any{"sample":sample,"config":batteryCfg,"error":fmt.Sprint(e)});fmt.Println(string(outJSON));return};if len(os.Args)>1&&os.Args[1]=="--battery-release"{loadBatteryConfig();if batteryCfg.Paused||batteryCfg.ManualPaused{if e:=batteryDriverSetPaused(false);e!=nil{log.Fatal(e)}};batteryCfg.Enabled=false;batteryCfg.ManualPaused=false;batteryCfg.Paused=false;if e:=saveBatteryConfig(batteryCfg);e!=nil{log.Fatal(e)};fmt.Println("Charging policy released");return};if b,e:=os.ReadFile("/data/airlink/sessions.json");e==nil{_ = json.Unmarshal(b,&sessions)};listen:="192.168.100.1:8780";if s:=os.Getenv("CONSOLE_LISTEN");s!=""{listen=s};if len(os.Args)>1&&os.Args[1]=="--check"{v,e:=status();if e!=nil{log.Fatal(e)};n,_:=v["native"].(map[string]any);outJSON,_:=json.Marshal(map[string]any{"native_status":n!=nil,"network_type":n["network_type"],"version":"2.0","lte_lock_interface":"QMI NAS"});fmt.Println(string(outJSON));return};startBatteryController();startSubscriptionMonitor();h:=http.HandlerFunc(handler);srv:=&http.Server{Addr:listen,Handler:h,ReadHeaderTimeout:5*time.Second,ReadTimeout:15*time.Second,WriteTimeout:20*time.Second,IdleTimeout:40*time.Second,MaxHeaderBytes:16<<10};ln,e:=net.Listen("tcp",listen);if e!=nil{log.Fatal(e)};log.Printf("Device console listening on %s",listen);log.Fatal(srv.Serve(ln))}
