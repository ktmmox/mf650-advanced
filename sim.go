package main
import("context";"encoding/json";"net/http";"os";"os/exec";"strconv";"strings";"sync";"time")
var simMu sync.Mutex
var simPending bool
var simMessage string
var simTarget int
func validSIMSlot(slot int)bool{return slot>=1&&slot<=3}
func simName(slot int)string{switch slot{case 1:return "内置卡";case 2:return "内置副卡";case 3:return "外置卡"};return "暂未确认"}
// Native network_mgr uses physical slot 1 for external, slot 2 for internal;
// GPIO23 selects internal main (0) or secondary (1). Its sim argument is 1/2/3.
func logicalSIMSlot(physical int,gpio string)int{if physical==1{return 3};if physical==2{switch strings.TrimSpace(gpio){case "0":return 1;case "1":return 2}};return 0}
func currentSIMSlot()int{physical,_:=strconv.Atoi(nv("modem_sim_slot"));gpio,_:=os.ReadFile("/sys/class/gpio/gpio23/value");return logicalSIMSlot(physical,string(gpio))}
func simSelectionPolicy()int{boot,_:=os.ReadFile("/etc/usb/boot_hsusb_comp");key:="dial_policy_temp";if strings.HasPrefix(strings.TrimSpace(string(boot)),"9057"){key="dial_policy"};p,e:=strconv.Atoi(nv(key));if e!=nil{return -1};return p}
func simTargetMatches(slot,policy,target int)bool{return slot==target&&((target==3&&policy==0)||(target!=3&&policy==1))}
func invalidateSubscription(){diagSubMu.Lock();diagSub=DiagSubscription{};_ = os.Remove(diagSubscriptionPath);diagSubMu.Unlock();subMu.Lock();subscription=map[string]any{"available":false,"message":"SIM 卡正在切换，等待新卡下发签约参数。"};subAt=time.Now();subMu.Unlock()}
func nativeSIMReplyOK(raw []byte)bool{var result struct{RC *int `json:"rc"`};return json.Unmarshal(raw,&result)==nil&&result.RC!=nil&&*result.RC==0}
func switchNativeSIM(target int){modemMu.Lock();defer modemMu.Unlock();invalidateSubscription();ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel();raw,e:=exec.CommandContext(ctx,"ubus","call","network_mgr","set_internal_simcard",`{"sim":`+strconv.Itoa(target)+`}`).CombinedOutput();message:="固件未确认切换，请刷新实际卡位后再操作。";if e==nil&&nativeSIMReplyOK(raw){deadline:=time.Now().Add(15*time.Second);verified:=false;for time.Now().Before(deadline){if simTargetMatches(currentSIMSlot(),simSelectionPolicy(),target){verified=true;break};time.Sleep(time.Second)};if verified{message="已回读确认选择"+simName(target)+"；请查看联网状态，空卡或未激活的卡可能无法注册网络。"}else{message="固件已应答，但实际卡位尚未确认，请刷新状态，不要连续切换。"}};simMu.Lock();simMessage=message;simPending=false;simMu.Unlock()}
func simAPI(w http.ResponseWriter,r *http.Request){if r.Method=="GET"{slot:=currentSIMSlot();if demo{slot=3};simMu.Lock();pending,message,target:=simPending,simMessage,simTarget;simMu.Unlock();out(w,map[string]any{"slot":slot,"name":simName(slot),"supported":slot!=0,"pending":pending,"message":message,"target":target});return};if r.Method!="POST"{fail(w,405,"请使用切换按钮");return};var b struct{Slot int `json:"slot"`};if body(r,&b)!=nil||!validSIMSlot(b.Slot){fail(w,400,"请选择内置卡、内置副卡或外置卡。");return};if demo{out(w,map[string]any{"ok":true,"message":"预览模式不会实际切卡。"});return};simMu.Lock();defer simMu.Unlock();if simPending{fail(w,409,"正在切换，请等待状态回读。");return};old:=currentSIMSlot();if old==0{fail(w,502,"无法确认实际卡位，未发送切换请求。");return};if simTargetMatches(old,simSelectionPolicy(),b.Slot){out(w,map[string]any{"ok":true,"message":"当前已选择"+simName(b.Slot)+"，无需重复切换。"});return};simPending=true;simTarget=b.Slot;simMessage="正在通过原固件切换至"+simName(b.Slot)+"，请等待重新驻网…";go switchNativeSIM(b.Slot);out(w,map[string]any{"ok":true,"pending":true,"message":simMessage})}
