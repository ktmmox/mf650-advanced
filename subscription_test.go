package main
import "testing"
func TestQoSParsing(t *testing.T){d:=parseQoS("AT+CGEQOSRDP\r\n+CGEQOSRDP: 1,9,0,0,0,0,500000,100000\r\nOK");if d["qci"]!=float64(9)||d["down_kbps"]!=float64(500000)||d["up_kbps"]!=float64(100000){t.Fatal(d)}}
func TestQoSDoesNotInventValues(t *testing.T){d:=parseQoS("AT+CGEQOSRDP\r\nOK");if d["available"]!=false||d["qci"]!=nil||d["down_kbps"]!=nil{t.Fatal(d)}}

func TestQoSEmptyHasExplanation(t *testing.T){d:=parseQoS("OK");if d["response_state"]!="empty"||d["source"]!="AT+CGEQOSRDP"||d["message"]==""{t.Fatal(d)}}

func TestActiveContext(t *testing.T){cid,apn:=activeContext("+CGCONTRDP: 1,5,ctlte,10.0.0.1\r\nOK");if cid!=1||apn!="ctlte"{t.Fatal(cid,apn)};cid,_=activeContext("OK");if cid!=0{t.Fatal(cid)}}
