package handlers

import (
	"strings"
)

type CommandRegistry struct {
	Responses map[string]string
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		Responses: map[string]string{
			"topic_not_found":        "Топик «{topic}» не найден. Доступные: {topics}",
			"topic_not_found_simple": "Не понял топик. Доступные: {topics}",
			"no_topics":              "Нет зарегистрированных топиков.",
			"id_result":              "🆔 ID этого топика: {id}",
			"id_general":             "📋 Это общий чат, у него нет ID топика.",
			"resend_usage":           "Укажи топик: /resend [название или slug]",
			"resend_no_reply":        "Ответь на сообщение, которое хочешь переслать.",
			"resend_ok":              "↗️ Продублировал в «{topic}»",
			"register_usage":         "Укажи название: /register [название топика]",
			"register_general":       "❌ Это общий чат. Напиши /register в нужном топике.",
			"register_exists":        "⚠️ Этот топик уже зарегистрирован.",
			"register_ok":            "✅ Топик «{name}» зарегистрирован (ID: {id})",
			"register_error":         "❌ Ошибка при регистрации топика.",
			"unknown_command":        "Неизвестная команда. Напиши /help чтобы увидеть список.",
		},
	}
}

func (r *CommandRegistry) Response(key string, params map[string]string) string {
	if r == nil || r.Responses == nil {
		return key
	}
	tpl, ok := r.Responses[key]
	if !ok {
		return key
	}
	for k, v := range params {
		tpl = strings.ReplaceAll(tpl, "{"+k+"}", v)
	}
	return tpl
}
