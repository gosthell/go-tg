package tgb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	tg "github.com/mr-linch/go-tg"
	"github.com/stretchr/testify/assert"
)

func testWithClientLocal(
	t *testing.T,
	do func(t *testing.T, ctx context.Context, client *tg.Client),
	handler http.HandlerFunc,
) {
	t.Helper()

	server := httptest.NewServer(handler)
	defer server.Close()

	client := tg.New("12345:secret",
		tg.WithServer(server.URL),
		tg.WithDoer(http.DefaultClient),
	)

	ctx := context.Background()

	do(t, ctx, client)
}

func TestAny(t *testing.T) {
	var (
		filterYes = FilterFunc(func(ctx context.Context, update *tg.Update) (bool, error) {
			return true, nil
		})
		filterNo = FilterFunc(func(ctx context.Context, update *tg.Update) (bool, error) {
			return false, nil
		})
		filterErr = FilterFunc(func(ctx context.Context, update *tg.Update) (bool, error) {
			return false, errors.New("some error")
		})
	)

	allow, err := Any(filterYes, filterNo).Allow(context.Background(), &tg.Update{})
	assert.NoError(t, err)
	assert.True(t, allow)

	allow, err = Any(filterNo, filterNo).Allow(context.Background(), &tg.Update{})
	assert.NoError(t, err)
	assert.False(t, allow)

	allow, err = Any(filterErr, filterYes).Allow(context.Background(), &tg.Update{})
	assert.Error(t, err)
	assert.False(t, allow)
}

func TestAll(t *testing.T) {
	var (
		filterYes = FilterFunc(func(ctx context.Context, update *tg.Update) (bool, error) {
			return true, nil
		})
		filterNo = FilterFunc(func(ctx context.Context, update *tg.Update) (bool, error) {
			return false, nil
		})
		filterErr = FilterFunc(func(ctx context.Context, update *tg.Update) (bool, error) {
			return false, errors.New("some error")
		})
	)

	allow, err := All(filterYes, filterYes).Allow(context.Background(), &tg.Update{})
	assert.NoError(t, err)
	assert.True(t, allow)

	allow, err = All(filterYes, filterNo).Allow(context.Background(), &tg.Update{})
	assert.NoError(t, err)
	assert.False(t, allow)

	allow, err = All(filterYes, filterErr).Allow(context.Background(), &tg.Update{})
	assert.Error(t, err)
}

func TestCommandFilter(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		Name    string
		Command *CommandFilter
		Update  *tg.Update
		Allow   bool
		Error   error
	}{
		{
			Name:    "Default",
			Command: Command("start"),
			Update: &tg.Update{
				Message: &tg.Message{
					Text: "/start azcv 5678",
				},
			},
			Allow: true,
		},
		{
			Name:    "NotMessage",
			Command: Command("start"),
			Update:  &tg.Update{},
			Allow:   false,
		},
		{
			Name: "InCaption",
			Command: Command("start",
				WithCommandIgnoreCaption(false),
			),
			Update: &tg.Update{
				Message: &tg.Message{
					Caption: "/start azcv 5678",
				},
			},
			Allow: true,
		},
		{
			Name: "NoTextOrCaption",
			Command: Command("start",
				WithCommandIgnoreCaption(false),
			),
			Update: &tg.Update{
				Message: &tg.Message{},
			},
			Allow: false,
		},
		{
			Name:    "BadPrefix",
			Command: Command("start"),
			Update: &tg.Update{
				Message: &tg.Message{
					Text: "!start azcv 5678",
				},
			},
			Allow: false,
		},
		{
			Name: "CustomPrefix",
			Command: Command("start",
				WithCommandPrefix("!"),
			),
			Update: &tg.Update{
				Message: &tg.Message{
					Text: "!start azcv 5678",
				},
			},
			Allow: true,
		},
		{
			Name:    "WithSelfMention",
			Command: Command("start"),
			Update: &tg.Update{
				Message: &tg.Message{
					Text: "/start@go_tg_test_bot azcv 5678",
				},
			},
			Allow: true,
		},
		{
			Name:    "WithNotSelfMention",
			Command: Command("start"),
			Update: &tg.Update{
				Message: &tg.Message{
					Text: "/start@anybot azcv 5678",
				},
			},
			Allow: false,
		},
		{
			Name:    "NotRegisteredCommand",
			Command: Command("start"),
			Update: &tg.Update{
				Message: &tg.Message{
					Text: "/help azcv 5678",
				},
			},
			Allow: false,
		},
		{
			Name:    "WithNotSelfMentionAndIgnore",
			Command: Command("start", WithCommandIgnoreMention(true)),
			Update: &tg.Update{
				Message: &tg.Message{
					Text: "/start@anybot azcv 5678",
				},
			},
			Allow: true,
		},
		{
			Name:    "WithIgnoreCase",
			Command: Command("start", WithCommandIgnoreCase(false)),
			Update: &tg.Update{
				Message: &tg.Message{
					Text: "/START azcv 5678",
				},
			},
			Allow: false,
		},
		{
			Name:    "WithAlias",
			Command: Command("start", WithCommandAlias("help")),
			Update: &tg.Update{
				Message: &tg.Message{
					Text: "/help azcv 5678",
				},
			},
			Allow: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			testWithClientLocal(t, func(t *testing.T, ctx context.Context, client *tg.Client) {
				update := *test.Update

				update.Bind(client)

				allow, err := test.Command.Allow(ctx, &update)
				assert.Equal(t, test.Allow, allow)
				assert.Equal(t, test.Error, err)
			}, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/bot12345:secret/getMe", r.URL.Path)

				w.WriteHeader(http.StatusOK)

				w.Write([]byte(`{
					"ok": true,
					"result": {
						"id": 5433024556,
						"is_bot": true,
						"first_name": "go-tg: test bot",
						"username": "go_tg_test_bot",
						"can_join_groups": true,
						"can_read_all_group_messages": true,
						"supports_inline_queries": true
					}
				}`))
			})
		})
	}
}