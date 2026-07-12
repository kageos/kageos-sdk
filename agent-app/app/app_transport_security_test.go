package app

import (
	"context"
	"testing"

	"github.com/kageos/kageos-sdk/dto"
	"github.com/kageos/kageos-sdk/pkg/contextx"
)

func TestBuildMessageCommandDoesNotExposeToken(t *testing.T) {
	ctx := contextx.WithToken(context.Background(), "must-not-enter-message-nats")
	ctx = contextx.WithRequestUser(ctx, "alice")
	msg, err := buildMessageCommand(ctx, &dto.MessageSendEnvelope{})
	if err != nil {
		t.Fatal(err)
	}
	if got := msg.Header.Get(contextx.TokenHeader); got != "" {
		t.Fatalf("message NATS header leaked token %q", got)
	}
	if got := msg.Header.Get(contextx.RequestUserHeader); got != "alice" {
		t.Fatalf("audit identity = %q, want alice", got)
	}
}
