package main
import("net/http";"os";"strconv";"strings";"sync";"time")
var memoryCleanMu sync.Mutex
var memoryCleanAt time.Time
func availableMemoryKB()uint64{raw,_:=os.ReadFile("/proc/meminfo");for _,line:=range strings.Split(string(raw),"\n"){f:=strings.Fields(line);if len(f)>1&&f[0]=="MemAvailable:"{v,_:=strconv.ParseUint(f[1],10,64);return v}};return 0}
func memoryCleanAPI(w http.ResponseWriter,r *http.Request){if r.Method!="POST"{fail(w,405,"请使用清理按钮");return};memoryCleanMu.Lock();defer memoryCleanMu.Unlock();if time.Since(memoryCleanAt)<30*time.Second{fail(w,429,"刚刚已清理，请 30 秒后再试。");return};if demo{out(w,map[string]any{"ok":true,"message":"预览模式：实际设备会释放可回收文件缓存。"});return};before:=availableMemoryKB();if _,e:=cmd("sync");e!=nil{fail(w,502,"缓存同步未完成，请稍后再试。");return};if e:=os.WriteFile("/proc/sys/vm/drop_caches",[]byte("1\n"),0200);e!=nil{fail(w,502,"固件未允许清理文件缓存。");return};memoryCleanAt=time.Now();after:=availableMemoryKB();out(w,map[string]any{"ok":true,"message":"文件缓存已清理；系统会根据需要重新使用内存。","before_available_kb":before,"after_available_kb":after})}
