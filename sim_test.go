package main
import "testing"
func TestValidSIMSlot(t *testing.T){for _,v:=range []int{-1,0,4,99}{if validSIMSlot(v){t.Fatalf("invalid slot accepted: %d",v)}};for _,v:=range []int{1,2,3}{if !validSIMSlot(v){t.Fatal("native SIM target rejected")}}}
func TestLogicalSIMSlot(t *testing.T){for _,c:=range []struct{physical int;gpio string;want int}{{1,"0",3},{1,"1",3},{2,"0\n",1},{2,"1\n",2},{2,"",0},{0,"0",0},{3,"0",0}}{if got:=logicalSIMSlot(c.physical,c.gpio);got!=c.want{t.Fatalf("%+v got %d",c,got)}}}
func TestSIMTargetVerification(t *testing.T){if !simTargetMatches(3,0,3)||!simTargetMatches(1,1,1)||!simTargetMatches(2,1,2){t.Fatal("matching state rejected")};if simTargetMatches(3,2,3)||simTargetMatches(1,1,2)||simTargetMatches(2,0,2){t.Fatal("unconfirmed target accepted")}}
func TestNativeSIMReply(t *testing.T){for _,s:=range []string{`{}`,`{"rc":1}`,`{"rc":null}`,`OK`}{if nativeSIMReplyOK([]byte(s)){t.Fatal("invalid reply accepted")}};if !nativeSIMReplyOK([]byte(`{"rc":0}`)){t.Fatal("valid reply rejected")}}
