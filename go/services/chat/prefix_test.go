package chat

import "testing"

func TestParsePrefix(t *testing.T) {
	cases := []struct {
		in   string
		mode promptMode
		rest string
	}{
		{"@help how do i create a client", modeHelp, "how do i create a client"},
		{"  @HELP: where is it", modeHelp, "where is it"},
		{"@help", modeHelp, ""},
		{"@web who painted the mona lisa", modeWeb, "who painted the mona lisa"},
		{"@Web, hello", modeWeb, "hello"},
		{"@helpme please", modeData, "@helpme please"},
		{"@webinar dates", modeData, "@webinar dates"},
		{"how many clients", modeData, "how many clients"},
		{"please @help me", modeData, "please @help me"},
	}
	for _, c := range cases {
		mode, rest := parsePrefix(c.in)
		if mode != c.mode || rest != c.rest {
			t.Errorf("parsePrefix(%q) = (%v, %q), want (%v, %q)", c.in, mode, rest, c.mode, c.rest)
		}
	}
}

func TestMentionsModel(t *testing.T) {
	models := []string{"SalesOrder", "PhysioClient", "TreatmentPlan", "Employee"}
	yes := []string{"how many sales orders do i have", "show me the Treatment Plan", "list physio clients", "employees by salary"}
	no := []string{"what is the capital of france", "how do i plan a trip", "explain what an order book is", "who is the best client of all time"}
	for _, q := range yes {
		if _, ok := mentionsModel(q, models); !ok {
			t.Errorf("mentionsModel(%q) = false, want true", q)
		}
	}
	for _, q := range no {
		if m, ok := mentionsModel(q, models); ok {
			t.Errorf("mentionsModel(%q) matched %s, want no match", q, m)
		}
	}
}

func TestClassifierFailsClosed(t *testing.T) {
	for in, want := range map[string]bool{"NO": false, "no.": false, " No ": false, "YES": true, "yes": true, "": true, "maybe": true} {
		if got := classifierAsksAboutData(in); got != want {
			t.Errorf("classifierAsksAboutData(%q) = %v, want %v", in, got, want)
		}
	}
}
