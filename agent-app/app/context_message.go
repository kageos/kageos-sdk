package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/kageos/kageos-sdk/dto"
	"github.com/kageos/kageos-sdk/pkg/contextx"
)

type SendNotificationOpts struct {
	ToUsers     string `json:"to_users"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	ContentType string `json:"content_type"`
	Level       string `json:"level"`
	Files       string `json:"files,omitempty"`
}

func (c *Context) SendNotification(opts *SendNotificationOpts) error {
	if app == nil {
		return fmt.Errorf("app 未初始化")
	}
	if opts == nil {
		return fmt.Errorf("SendNotificationOpts 不能为 nil")
	}
	contentType := strings.ToLower(strings.TrimSpace(opts.ContentType))
	if contentType == "" {
		contentType = "markdown"
	}
	meta := c.messageSendMeta()
	toUsers := normalizeNotificationUsers(opts.ToUsers)
	if toUsers == "" {
		toUsers = defaultNotificationUser(meta.RequestUser)
	}
	envelope := &dto.MessageSendEnvelope{
		Meta: meta,
		Message: dto.MessageSendPayload{
			ToUsers:     toUsers,
			Title:       normalizeNotificationTitle(opts.Title, opts.Level),
			Content:     strings.TrimSpace(opts.Message),
			ContentType: contentType,
			Files:       normalizeNotificationFiles(opts.Files),
		},
	}
	return app.PublishMessage(c.Context, envelope)
}

type SendMessageOpts struct {
	ToUsers     string `json:"to_users"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Files       string `json:"files,omitempty"`
}

// Deprecated: use SendNotification.
func (c *Context) SendMessage(opts *SendMessageOpts) error {
	if opts == nil {
		return fmt.Errorf("SendMessageOpts 不能为 nil")
	}
	return c.SendNotification(&SendNotificationOpts{
		ToUsers:     opts.ToUsers,
		Title:       opts.Title,
		Message:     opts.Content,
		ContentType: opts.ContentType,
		Files:       opts.Files,
	})
}

func (c *Context) messageSendMeta() dto.MessageSendMeta {
	if c == nil {
		return dto.MessageSendMeta{From: "system"}
	}
	requestUser := strings.TrimSpace(c.GetRequestUser())
	from := requestUser
	if from == "" {
		from = "system"
	}
	sourceCtx := c.Context
	if sourceCtx == nil {
		sourceCtx = context.Background()
	}
	sourceType := strings.TrimSpace(contextx.GetSourceType(sourceCtx))
	sourceRef := strings.TrimSpace(contextx.GetSourceRef(sourceCtx))
	if sourceType == "" && sourceRef == "" {
		if fullCodePath := strings.TrimSpace(c.GetFullCodePath()); fullCodePath != "" {
			sourceType = "sdk_function"
			sourceRef = fullCodePath
		}
	}
	return dto.MessageSendMeta{
		From:                  from,
		RequestUser:           requestUser,
		DepartmentFullPath:    strings.TrimSpace(c.GetRequestUserDept()),
		FullCodePath:          strings.TrimSpace(c.GetFullCodePath()),
		TraceID:               strings.TrimSpace(c.GetTraceId()),
		ClientSource:          strings.TrimSpace(c.GetClientSource()),
		SourceType:            sourceType,
		SourceRef:             sourceRef,
		SourcePath:            firstNonEmpty(strings.TrimSpace(contextx.GetSourcePath(sourceCtx)), strings.TrimSpace(c.GetFullCodePath())),
		SourceTitle:           strings.TrimSpace(contextx.GetSourceTitle(sourceCtx)),
		SourceParentPath:      strings.TrimSpace(contextx.GetSourceParentPath(sourceCtx)),
		SourceParentTitle:     strings.TrimSpace(contextx.GetSourceParentTitle(sourceCtx)),
		SourceTemplateType:    strings.TrimSpace(contextx.GetSourceTemplateType(sourceCtx)),
		WorkspaceSessionID:    strings.TrimSpace(contextx.GetWorkspaceSessionID(sourceCtx)),
		WorkspaceSessionTitle: strings.TrimSpace(contextx.GetWorkspaceSessionTitle(sourceCtx)),
		WorkspaceRole:         strings.TrimSpace(contextx.GetWorkspaceRole(sourceCtx)),
	}
}

func normalizeNotificationTitle(title string, level string) string {
	title = strings.TrimSpace(title)
	switch normalizeNotificationLevel(level) {
	case "critical":
		if !strings.HasPrefix(title, "【高优先级】") && !strings.HasPrefix(title, "【紧急】") {
			title = "【高优先级】" + title
		}
	case "warning":
		if !strings.HasPrefix(title, "【提醒】") && !strings.HasPrefix(title, "【注意】") {
			title = "【提醒】" + title
		}
	}
	return title
}

func normalizeNotificationLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "critical", "warning", "info":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return "info"
	}
}

func normalizeNotificationUsers(toUsers string) string {
	return normalizeCommaSeparatedRefs(toUsers)
}

func normalizeNotificationFiles(files string) string {
	return normalizeCommaSeparatedRefs(files)
}

func normalizeCommaSeparatedRefs(value string) string {
	value = strings.NewReplacer(
		"，", ",",
		"、", ",",
		";", ",",
		"；", ",",
		"\n", ",",
		"\t", ",",
	).Replace(strings.TrimSpace(value))
	parts := strings.Split(value, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		ref := strings.TrimSpace(part)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	return strings.Join(out, ",")
}

func defaultNotificationUser(username string) string {
	username = strings.TrimSpace(username)
	if username == "" || username == "system" {
		return ""
	}
	return username
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
