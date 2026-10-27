package main
import "testing"
func TestATRejectsMultipleLines(t *testing.T){for _,s:=range []string{"AT\rAT+RESET","AT\nAT+RESET","echo hi","","AT\x00X"}{if validAT(s){t.Fatal("accepted invalid AT")}};if !validAT("AT+CSQ"){t.Fatal("rejected valid AT")}}
func TestIMEIMode(t *testing.T){if !validMode("imei"," ApplyIMEI ")||validMode("imei"," Apply "){t.Fatal("incorrect IMEI native action")}}
