package api

import (
	_ "omsu_bot/internal/db"
)

// @Summary Get JWT token
// @Description Authenticate with admin_secret and receive a JWT token
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body object false "admin_secret"
// @Success 200 {object} map[string]interface{}
// @Router /api/auth/token [post]
func _post_token_stub() {}

// @Summary Get persona
// @Description Get current bot persona settings
// @Tags Persona
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/persona [get]
func _get_persona_stub() {}

// @Summary Update persona
// @Description Update bot persona (name, system_prompt, signature)
// @Tags Persona
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body personaUpdateRequest false "Persona fields to update"
// @Success 200 {object} map[string]interface{}
// @Router /api/persona [put]
func _put_persona_stub() {}

// @Summary Reset persona
// @Description Reset persona to seed values from persona.md
// @Tags Persona
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/persona/reset [post]
func _post_persona_reset_stub() {}

// @Summary List topics
// @Description Get all forum topics with pagination
// @Tags Topics
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics [get]
func _get_topics_stub() {}

// @Summary Create topic
// @Description Create a new forum topic
// @Tags Topics
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body topicCreateRequest false "Topic data"
// @Success 201 {object} map[string]interface{}
// @Router /api/topics [post]
func _post_topics_stub() {}

// @Summary Get topic by ID
// @Description Get a single forum topic
// @Tags Topics
// @Security BearerAuth
// @Produce json
// @Param id path int true "Topic ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics/{id} [get]
func _get_topic_stub() {}

// @Summary Update topic
// @Description Update forum topic metadata
// @Tags Topics
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Topic ID"
// @Param body body topicUpdateRequest false "Fields to update"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics/{id} [put]
func _put_topic_stub() {}

// @Summary Delete topic
// @Description Delete a forum topic
// @Tags Topics
// @Security BearerAuth
// @Param id path int true "Topic ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics/{id}/delete [delete]
func _delete_topic_stub() {}

// @Summary Close topic
// @Description Close a forum topic
// @Tags Topics
// @Security BearerAuth
// @Param id path int true "Topic ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics/{id}/close [post]
func _post_topic_close_stub() {}

// @Summary Open topic
// @Description Reopen a closed forum topic
// @Tags Topics
// @Security BearerAuth
// @Param id path int true "Topic ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics/{id}/open [post]
func _post_topic_open_stub() {}

// @Summary Get permissions
// @Description Get command permissions matrix
// @Tags Permissions
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/permissions [get]
func _get_permissions_stub() {}

// @Summary Update permission
// @Description Update allowed_role for a command
// @Tags Permissions
// @Security BearerAuth
// @Accept json
// @Param command path string true "Command name"
// @Param body body updatePermissionRequest false "New allowed_role"
// @Success 200 {object} map[string]interface{}
// @Router /api/permissions/{command} [put]
func _put_permission_stub() {}

// @Summary Token usage stats
// @Description LLM token usage today by provider
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/tokens [get]
func _get_stats_tokens_stub() {}

// @Summary Request stats
// @Description LLM request count today by type
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/requests [get]
func _get_stats_requests_stub() {}

// @Summary Forward stats
// @Description Message forward statistics
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/forwards [get]
func _get_stats_forwards_stub() {}

// @Summary Message stats
// @Description Processed message statistics
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/messages [get]
func _get_stats_messages_stub() {}

// @Summary Provider stats
// @Description LLM provider performance stats
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/providers [get]
func _get_stats_providers_stub() {}

// @Summary Schedule snapshots
// @Description Schedule change snapshot history with pagination
// @Tags Schedule
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /api/schedule/snapshots [get]
func _get_schedule_snapshots_stub() {}

// @Summary Schedule anomalies
// @Description Detected schedule anomalies with pagination
// @Tags Schedule
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /api/schedule/anomalies [get]
func _get_schedule_anomalies_stub() {}

// @Summary List groups
// @Description Get all groups
// @Tags Groups
// @Security BearerAuth
// @Produce json
// @Success 200 {array} db.Group
// @Router /api/groups [get]
func _get_groups_stub() {}

// @Summary Create group
// @Description Create a new group
// @Tags Groups
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body groupRequest true "Group data"
// @Success 201 {object} db.Group
// @Router /api/groups [post]
func _post_groups_stub() {}

// @Summary Get group by chat ID
// @Description Get a single group configuration
// @Tags Groups
// @Security BearerAuth
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Success 200 {object} db.Group
// @Router /api/groups/{chat_id} [get]
func _get_group_stub() {}

// @Summary Update group
// @Description Update group configuration fields
// @Tags Groups
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Param body body groupRequest true "Group fields to update"
// @Success 200 {object} db.Group
// @Router /api/groups/{chat_id} [put]
func _put_group_stub() {}

// @Summary Delete group
// @Description Delete group configuration
// @Tags Groups
// @Security BearerAuth
// @Param chat_id path int true "Group chat ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/groups/{chat_id} [delete]
func _delete_group_stub() {}

// @Summary Get group persona file content
// @Description Read group-specific persona.md content
// @Tags Groups
// @Security BearerAuth
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/groups/{chat_id}/context/persona [get]
func _get_group_persona_stub() {}

// @Summary Upload group persona file content
// @Description Write/upload group-specific persona.md content
// @Tags Groups
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Param body body contextUploadRequest true "File content"
// @Success 200 {object} map[string]interface{}
// @Router /api/groups/{chat_id}/context/persona [put]
func _put_group_persona_stub() {}

// @Summary Get group system prompt content
// @Description Read group-specific system_prompt.txt content
// @Tags Groups
// @Security BearerAuth
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/groups/{chat_id}/context/system-prompt [get]
func _get_group_system_prompt_stub() {}

// @Summary Upload group system prompt content
// @Description Write/upload group-specific system_prompt.txt content
// @Tags Groups
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Param body body contextUploadRequest true "File content"
// @Success 200 {object} map[string]interface{}
// @Router /api/groups/{chat_id}/context/system-prompt [put]
func _put_group_system_prompt_stub() {}

// @Summary Get group knowledge base content
// @Description Read group-specific knowledge_base.txt content
// @Tags Groups
// @Security BearerAuth
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/groups/{chat_id}/context/knowledge [get]
func _get_group_knowledge_stub() {}

// @Summary Upload group knowledge base content
// @Description Write/upload group-specific knowledge_base.txt content
// @Tags Groups
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Param body body contextUploadRequest true "File content"
// @Success 200 {object} map[string]interface{}
// @Router /api/groups/{chat_id}/context/knowledge [put]
func _put_group_knowledge_stub() {}

// @Summary Get group feature flags
// @Description Read group-specific feature configurations
// @Tags Groups
// @Security BearerAuth
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/groups/{chat_id}/context/features [get]
func _get_group_features_stub() {}

// @Summary Upload group feature flags
// @Description Write/update group-specific feature configurations
// @Tags Groups
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param chat_id path int true "Group chat ID"
// @Param body body map[string]bool true "Features map"
// @Success 200 {object} map[string]interface{}
// @Router /api/groups/{chat_id}/context/features [put]
func _put_group_features_stub() {}

// @Summary List superadmins
// @Description Get all registered superadmins
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {array} db.Superadmin
// @Router /api/admin/superadmins [get]
func _get_superadmins_stub() {}

// @Summary Add superadmin
// @Description Add a new user as a superadmin
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body superadminAddRequest true "Superadmin data"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/superadmins [post]
func _post_superadmin_stub() {}

// @Summary Remove superadmin
// @Description Remove superadmin status from a user
// @Tags Admin
// @Security BearerAuth
// @Param user_id path int true "Telegram user ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/superadmins/{user_id} [delete]
func _delete_superadmin_stub() {}

// @Summary Register webhooks with Setka
// @Description Re-register webhook configurations for all active groups in Setka
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/groups/register-webhooks [post]
func _post_register_webhooks_stub() {}
