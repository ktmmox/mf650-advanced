package main

import (
 "encoding/binary"
 "encoding/json"
 "errors"
 "fmt"
 "net/http"
 "os"
 "strconv"
 "strings"
 "sync"
 "time"
)

const batteryConfigPath="/data/airlink/battery.json"
var batteryPercents=[]int{0,15,25,50,75,90,100}
type BatteryConfig struct{Enabled bool `json:"enabled"`;Stop int `json:"stop_percent"`;Resume int `json:"resume_percent"`;ManualPaused bool `json:"manual_paused"`;Paused bool `json:"paused"`}
type BatterySample struct{Supported bool `json:"supported"`;Level int `json:"level"`;Percent int `json:"percent_estimate"`;Charging bool `json:"charging"`;CurrentRaw int `json:"-"`;PowerMode int `json:"power_mode"`;Full bool `json:"full"`}
type BatteryEvent struct{Time int64 `json:"time"`;Message string `json:"message"`}
var batteryMu sync.Mutex
var batteryCfg=BatteryConfig{Stop:90,Resume:75}
var batteryError string
var batteryPending bool
var batteryForceResume bool
var batteryEvents=[]BatteryEvent{}
var batteryWake=make(chan struct{},1)

func decodeBattery(raw []byte,level int)(BatterySample,error){
 if len(raw)<32{return BatterySample{},errors.New("电源芯片返回的数据不完整")}
 if level<1||level>6{return BatterySample{},errors.New("原厂电量读数暂不可用")}
 current:=int(int16(binary.LittleEndian.Uint16(raw[24:26])))
 return BatterySample{Supported:true,Level:level,Percent:batteryPercents[level],Charging:current>160,CurrentRaw:current,PowerMode:int(raw[13]&7),Full:level==6&&current<=160},nil
}
func readBattery()(BatterySample,error){raw,e:=batteryDriverRead();if e!=nil{return BatterySample{},e};level,e:=strconv.Atoi(strings.TrimSpace(nv("bat_value")));if e!=nil{return BatterySample{},errors.New("无法读取原厂电量档位")};return decodeBattery(raw,level)}
func validateBatteryConfig(c BatteryConfig)error{valid:=func(x int)bool{for _,p:=range batteryPercents[1:]{if x==p{return true}};return false};if !valid(c.Stop)||!valid(c.Resume)||c.Resume>=c.Stop||c.Stop<50{return errors.New("请选择有效电量档位：停充至少 50%，恢复电量必须低于停充电量")};return nil}
func desiredBatteryPause(c BatteryConfig,percent int)bool{if c.ManualPaused{return true};if !c.Enabled{return false};if percent>=c.Stop{return true};if percent<=c.Resume{return false};return c.Paused}
func saveBatteryConfig(c BatteryConfig)error{b,e:=json.Marshal(c);if e!=nil{return e};tmp:=batteryConfigPath+".tmp";if e=os.WriteFile(tmp,b,0600);e!=nil{return e};return os.Rename(tmp,batteryConfigPath)}
func loadBatteryConfig(){b,e:=os.ReadFile(batteryConfigPath);if e!=nil{if !os.IsNotExist(e){batteryError="无法读取已保存的充电策略"};return};var c BatteryConfig;if e=json.Unmarshal(b,&c);e!=nil||validateBatteryConfig(c)!=nil{batteryError="充电策略文件无效，自动控制未启用";return};batteryCfg=c}
func addBatteryEvent(s string){batteryEvents=append([]BatteryEvent{{Time:time.Now().Unix(),Message:s}},batteryEvents...);if len(batteryEvents)>10{batteryEvents=batteryEvents[:10]}}
func wakeBattery(){select{case batteryWake<-struct{}{}:default:}}
func batteryTick(){
 batteryMu.Lock();defer batteryMu.Unlock()
 if batteryForceResume{if e:=batteryDriverSetPaused(false);e!=nil{batteryError="恢复充电失败："+e.Error();batteryPending=false;return};time.Sleep(3*time.Second);batteryForceResume=false;batteryCfg.Paused=false;batteryPending=false;batteryError="";addBatteryEvent("已允许充电，实际充电状态由电源芯片回报");if e:=saveBatteryConfig(batteryCfg);e!=nil{batteryError="恢复充电已执行，但配置保存失败"};return}
 if !batteryCfg.Enabled&&!batteryCfg.ManualPaused&&!batteryCfg.Paused{batteryPending=false;return}
 sample,e:=readBattery();if e!=nil{batteryError=e.Error();batteryPending=false;if batteryCfg.Enabled&&batteryCfg.Paused{if err:=batteryDriverSetPaused(false);err==nil{batteryCfg.Paused=false;_ = saveBatteryConfig(batteryCfg);addBatteryEvent("电量读取异常，已释放自动停充")}};return}
 target:=desiredBatteryPause(batteryCfg,sample.Percent)
 if target==batteryCfg.Paused&&(!target||!sample.Charging){batteryPending=false;return}
 if e=batteryDriverSetPaused(target);e!=nil{batteryError="充电控制未执行："+e.Error();batteryPending=false;return}
 // IP5332 current measurement settles after the same delay used by the factory service.
 time.Sleep(3*time.Second)
 after,e:=readBattery()
 if e!=nil||(target&&after.Charging){
  if target&&!batteryCfg.Paused{_ = batteryDriverSetPaused(false)}
  batteryError="充电状态未通过回读校验，请重新读取或恢复充电";batteryPending=false;addBatteryEvent(batteryError);return
 }
 batteryCfg.Paused=target;batteryPending=false;batteryError=""
 if target{addBatteryEvent(fmt.Sprintf("已暂停充电，当前电量约 %d%%",after.Percent))}else{addBatteryEvent(fmt.Sprintf("已允许充电，当前电量约 %d%%",after.Percent))}
 if e=saveBatteryConfig(batteryCfg);e!=nil{batteryError="控制已执行，但策略状态保存失败"}
}
func startBatteryController(){loadBatteryConfig();go func(){batteryTick();ticker:=time.NewTicker(10*time.Second);defer ticker.Stop();for{select{case <-ticker.C:batteryTick();case <-batteryWake:batteryTick()}}}()}
func batteryAPI(w http.ResponseWriter,r *http.Request){
 switch r.URL.Path{
 case "/api/battery":
  if r.Method!="GET"{fail(w,405,"请读取电池状态");return};batteryMu.Lock();defer batteryMu.Unlock();sample,e:=readBattery();notice:=batteryError;if e!=nil{notice=e.Error()};_,driverErr:=os.Stat("/dev/IP5332");out(w,map[string]any{"control_supported":driverErr==nil,"sample":sample,"config":batteryCfg,"error":notice,"pending":batteryPending,"events":batteryEvents,"poll_seconds":10,"percent_source":"原厂六档电量估算"})
 case "/api/battery/settings":
  if r.Method!="POST"{fail(w,405,"请使用保存按钮");return};var request struct{Enabled bool `json:"enabled"`;Stop int `json:"stop_percent"`;Resume int `json:"resume_percent"`};if body(r,&request)!=nil{fail(w,400,"请求无效");return};candidate:=BatteryConfig{Enabled:request.Enabled,Stop:request.Stop,Resume:request.Resume};if e:=validateBatteryConfig(candidate);e!=nil{fail(w,400,e.Error());return};if _,e:=readBattery();e!=nil{fail(w,501,e.Error());return};batteryMu.Lock();candidate.Paused=batteryCfg.Paused;if e:=saveBatteryConfig(candidate);e!=nil{batteryMu.Unlock();fail(w,500,"策略保存失败，尚未更改充电控制");return};batteryCfg=candidate;batteryPending=true;batteryError="";if candidate.Enabled{addBatteryEvent(fmt.Sprintf("自动保护已开启：约 %d%% 停充，约 %d%% 恢复",candidate.Stop,candidate.Resume))}else{addBatteryEvent("自动保护已关闭，恢复正常充电管理")};batteryMu.Unlock();wakeBattery();out(w,map[string]any{"ok":true,"message":"策略已保存，后台正在检测并校验充电状态。"})
 case "/api/battery/control":
  if r.Method!="POST"{fail(w,405,"请使用控制按钮");return};var b struct{Action string};if body(r,&b)!=nil||(b.Action!="pause"&&b.Action!="resume"){fail(w,400,"仅支持暂停或恢复充电");return};if _,e:=batteryDriverRead();e!=nil{fail(w,501,e.Error());return};batteryMu.Lock();c:=batteryCfg;c.Enabled=false;c.ManualPaused=b.Action=="pause";if e:=saveBatteryConfig(c);e!=nil{batteryMu.Unlock();fail(w,500,"控制设置保存失败");return};batteryCfg=c;batteryForceResume=b.Action=="resume";batteryPending=true;batteryError="";if c.ManualPaused{addBatteryEvent("已请求手动暂停充电，自动保护已关闭")}else{addBatteryEvent("已请求恢复充电，自动保护已关闭")};batteryMu.Unlock();wakeBattery();out(w,map[string]any{"ok":true,"message":"控制请求已保存，正在执行并回读验证。"})
 default:fail(w,404,"电池接口不存在")
 }
}
