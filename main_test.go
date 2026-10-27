package main
import("testing";"time";"net/http/httptest";"net/http";"strings")
func TestNativeStatusParsing(t *testing.T){m,e:=parseStatus(`var si_new={network_type:"NR5G-NSA",cpu:{busy:0xb30c,total:0x1fede},ram:{total:227888,used:204304}};`);if e!=nil{t.Fatal(e)};if m["network_type"]!="NR5G-NSA"{t.Fatal(m)};c:=m["cpu"].(map[string]any);if c["busy"]!=float64(45836){t.Fatal(c)}}
func TestMalformedStatusRejected(t *testing.T){if _,e:=parseStatus("Not found");e==nil{t.Fatal("accepted invalid status")}}
func TestModePerPage(t *testing.T){if validMode("lan"," Apply ")||!validMode("lan"," Restart ")||validMode("bands"," Apply ")||!validMode("sleep"," ApplyAutoSleep "){t.Fatal("incorrect native mode mapping")}}
func TestFixedHiddenValues(t *testing.T){s:=`<input name="action_script" value="restart_wifis"><input name='sid_list' value='WLANConfig11a;'>`;if hiddenValue(s,"action_script")!="restart_wifis"||hiddenValue(s,"sid_list")!="WLANConfig11a;"{t.Fatal("hidden fields")}}
func TestRequiresAuthentication(t *testing.T){r:=httptest.NewRequest(http.MethodGet,"/api/status",nil);w:=httptest.NewRecorder();handler(w,r);if w.Code!=401{t.Fatal(w.Code)}}
func TestRejectsCSRF(t *testing.T){id:="test-session";sessions[id]=session{Expires:future(),CSRF:"test-csrf"};r:=httptest.NewRequest(http.MethodPost,"/api/save",strings.NewReader(`{}`));r.AddCookie(&http.Cookie{Name:"console_session",Value:id});w:=httptest.NewRecorder();handler(w,r);if w.Code!=403{t.Fatal(w.Code)};delete(sessions,id)}
func future() time.Time{return time.Now().Add(time.Hour)}

