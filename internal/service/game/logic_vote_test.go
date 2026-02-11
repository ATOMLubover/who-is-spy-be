package game

import (
	"testing"
)

func TestVoteStageHandler_PreventsDuplicateVotes(t *testing.T) {
	ctx := &GameContext{
		GameStage: STAGE_VOTING,
		Players: map[string]*Player{
			"player1": {ID: "player1", Name: "Alice", Role: ROLE_NORMAL},
			"player2": {ID: "player2", Name: "Bob", Role: ROLE_NORMAL},
		},
		Votes: make(map[string]string),
	}

	vsh := NewVoteStageHandler()

	firstReq := RequestWrapper{
		ReqType: REQ_VOTE,
		Data:    mustMarshal(VoteRequest{VoterID: "player1", TargetID: "player2"}),
	}

	if err := vsh.OnHandle(ctx, firstReq); err != nil {
		t.Fatalf("first vote should succeed, got: %v", err)
	}

	if got := ctx.Votes["player1"]; got != "player2" {
		t.Fatalf("vote not recorded correctly, want player2 got %q", got)
	}

	secondReq := RequestWrapper{
		ReqType: REQ_VOTE,
		Data:    mustMarshal(VoteRequest{VoterID: "player1", TargetID: "player2"}),
	}

	if err := vsh.OnHandle(ctx, secondReq); err == nil {
		t.Fatalf("duplicate vote should be rejected")
	}

	if len(ctx.Votes) != 1 {
		t.Fatalf("duplicate vote mutated votes map, want len=1 got %d", len(ctx.Votes))
	}
}
