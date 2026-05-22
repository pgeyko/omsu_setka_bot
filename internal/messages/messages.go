package messages

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Messages struct {
	InitGroupOnly       string `yaml:"init_group_only"`
	InitAdminOnly       string `yaml:"init_admin_only"`
	InitInvalidID       string `yaml:"init_invalid_id"`
	InitDBError         string `yaml:"init_db_error"`
	InitSuccess         string `yaml:"init_success"`
	ResendTopicNotFound string `yaml:"resend_topic_not_found"`
	ResendNoReply       string `yaml:"resend_no_reply"`
	ResendError         string `yaml:"resend_error"`
	RegisterUsage       string `yaml:"register_usage"`
	RegisterGeneral     string `yaml:"register_general"`
	RegisterExists      string `yaml:"register_exists"`
	RegisterError       string `yaml:"register_error"`
	RegisterOK          string `yaml:"register_ok"`
	TopicsHeader        string `yaml:"topics_header"`
	TopicsEmpty         string `yaml:"topics_empty"`
	TopicsClosed        string `yaml:"topics_closed"`
	TagError            string `yaml:"tag_error"`
	TagEmpty            string `yaml:"tag_empty"`
	TagUsage            string `yaml:"tag_usage"`
	IDTopic             string `yaml:"id_topic"`
	IDGeneral           string `yaml:"id_general"`
	HelpHeader          string `yaml:"help_header"`
	HelpDetail          string `yaml:"help_detail"`
	UnknownCommand      string `yaml:"unknown_command"`
}

var Default = defaultMessages()

func defaultMessages() *Messages {
	return &Messages{
		InitGroupOnly:       "❌ Команда /init работает только в группах или супергруппах.",
		InitAdminOnly:       "❌ Только администраторы могут использовать эту команду.",
		InitInvalidID:       "❌ Некорректный Omsu Group ID. Пожалуйста, укажите число.",
		InitDBError:         "❌ Ошибка при инициализации группы в базе данных.",
		InitSuccess:         "✅ Бот успешно инициализирован в этой группе!\n\nПожалуйста, отправьте по одному сообщению в каждый существующий топик, или переименуйте их, чтобы бот зафиксировал их в базе данных.",
		ResendTopicNotFound: "Топик «{topic}» не найден. Напиши /topics",
		ResendNoReply:       "Ответь на сообщение, которое хочешь переслать.",
		ResendError:         "Ошибка при пересылке.",
		RegisterUsage:       "Укажи название: /register [название топика]",
		RegisterGeneral:     "❌ Это общий чат. Напиши /register в нужном топике.",
		RegisterExists:      "⚠️ Этот топик уже зарегистрирован.",
		RegisterError:       "❌ Ошибка при регистрации топика.",
		RegisterOK:          "✅ Топик «{name}» зарегистрирован (ID: {id})",
		TopicsHeader:        "📋 <b>Топики:</b>",
		TopicsEmpty:         "Нет зарегистрированных топиков.",
		TopicsClosed:        "🔒",
		TagError:            "❌ Ошибка при поиске тега.",
		TagEmpty:            "Нет сообщений с тегом #{tag}.",
		TagUsage:            "Укажи тег: /tag #дедлайн",
		IDTopic:             "🆔 ID этого топика: {id}",
		IDGeneral:           "📋 Это общий чат, у него нет ID топика.",
		HelpHeader:          "🤖 <b>{name}</b> — ассистент группы",
		HelpDetail:          "Подробнее: @{username}",
		UnknownCommand:      "Неизвестная команда. Напиши /help чтобы увидеть список.",
	}
}

func Load(path string) *Messages {
	m := defaultMessages()
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	yaml.Unmarshal(data, m)
	return m
}

func (m *Messages) Format(template string, vars map[string]string) string {
	s := template
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}
