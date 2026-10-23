package ai

import (
	"strings"
	"testing"
)

func TestParseDecisionPlain(t *testing.T) {
	d, err := ParseDecision(`{"thought":"look around","action":"keys","value":"<ENTER>"}`)
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionKeys || d.Value != "<ENTER>" {
		t.Errorf("unexpected decision: %+v", d)
	}
}

func TestParseDecisionFencedAndChatter(t *testing.T) {
	raw := "Sure! Here is the action:\n```json\n{\"thought\":\"open mail\",\"action\":\"Text\",\"value\":\"M\"}\n```\n"
	d, err := ParseDecision(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionText || d.Value != "M" {
		t.Errorf("unexpected decision: %+v", d)
	}
}

func TestParseDecisionWithBracesInString(t *testing.T) {
	d, err := ParseDecision(`{"thought":"post {curly} text","action":"text","value":"a } brace"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Value, "}") || d.Value != "a } brace" {
		t.Errorf("value = %q", d.Value)
	}
}

func TestParseDecisionDownloadDefaultsToZmodem(t *testing.T) {
	d, err := ParseDecision(`{"action":"download","thought":"get the file"}`)
	if err != nil {
		t.Fatal(err)
	}
	if d.Protocol != "zmodem" {
		t.Errorf("protocol = %q, want zmodem", d.Protocol)
	}
}

func TestParseDecisionRejectsUnknownAction(t *testing.T) {
	if _, err := ParseDecision(`{"action":"rm -rf","value":"/"}`); err == nil {
		t.Fatal("expected an error for an unknown action")
	}
}

func TestParseDecisionNoteWithoutAction(t *testing.T) {
	d, err := ParseDecision(`{"note":"the sysop is called Dave"}`)
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionNote {
		t.Errorf("action = %q, want note", d.Action)
	}
}

func TestTrimScreen(t *testing.T) {
	text := "one\ntwo\nthree\nfour"
	if got := TrimScreen(text, 2); !strings.Contains(got, "three") || strings.Contains(got, "one") {
		t.Errorf("trim = %q", got)
	}
	if got := TrimScreen(text, 10); got != text {
		t.Errorf("should not trim: %q", got)
	}
}
