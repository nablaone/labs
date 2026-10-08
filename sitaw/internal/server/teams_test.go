package server

import (
	"net/http"
	"testing"

	"sitaw/internal/store"
)

func TestTeamInviteAndCreate(t *testing.T) {
	ts, st, invite := setup(t)
	tok, _ := join(t, ts, invite, "probe")

	// The team's link is the one the member joined with.
	code, b := do(t, ts, "GET", "/api/team/invite", tok, "")
	if code != 200 {
		t.Fatalf("invite: %d %s", code, b)
	}
	if got := decode[struct{ Token string }](t, b).Token; got != invite {
		t.Fatalf("invite token = %q, want %q", got, invite)
	}
	if code, _ := do(t, ts, "GET", "/api/team/invite", "nope", ""); code != http.StatusUnauthorized {
		t.Fatalf("no session: %d", code)
	}

	// Agents get neither.
	_, b = do(t, ts, "POST", "/api/agents", tok, "")
	ag := decode[createdAgent](t, b)
	if code, _ := do(t, ts, "GET", "/api/team/invite", ag.Token, ""); code != http.StatusForbidden {
		t.Fatalf("agent invite: %d", code)
	}
	if code, _ := do(t, ts, "POST", "/api/teams", ag.Token, `{"name":"x"}`); code != http.StatusForbidden {
		t.Fatalf("agent create team: %d", code)
	}

	// A person creates a team; its link works and the caller stays where they were.
	if code, _ := do(t, ts, "POST", "/api/teams", tok, `{"name":"  "}`); code != http.StatusBadRequest {
		t.Fatalf("blank name: %d", code)
	}
	code, b = do(t, ts, "POST", "/api/teams", tok, `{"name":"bravo"}`)
	if code != 200 {
		t.Fatalf("create team: %d %s", code, b)
	}
	created := decode[struct {
		Team  store.Team
		Token string
	}](t, b)
	if created.Team.Name != "bravo" || created.Token == "" {
		t.Fatalf("created = %+v", created)
	}
	if tm, err := st.InviteTeam(created.Token); err != nil || tm.ID != created.Team.ID {
		t.Fatalf("new invite leads to %+v, %v", tm, err)
	}
	_, b = do(t, ts, "GET", "/api/team/invite", tok, "")
	if got := decode[struct{ Token string }](t, b).Token; got != invite {
		t.Fatalf("caller moved teams: invite %q", got)
	}

	// With every link revoked there is none to share.
	if _, err := st.RevokeInvite(invite); err != nil {
		t.Fatal(err)
	}
	if code, _ := do(t, ts, "GET", "/api/team/invite", tok, ""); code != http.StatusNotFound {
		t.Fatalf("revoked invite: %d", code)
	}
}
