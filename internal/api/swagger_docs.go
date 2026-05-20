package api

// @Summary Get JWT token
// @Description Authenticate with admin_secret and receive a JWT token
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body object false "admin_secret"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/auth/token [post]

// @Summary Get persona
// @Description Get current bot persona settings
// @Tags Persona
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/persona [get]

// @Summary Update persona
// @Description Update bot persona (name, system_prompt, signature)
// @Tags Persona
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body personaUpdateRequest false "Persona fields to update"
// @Success 200 {object} map[string]interface{}
// @Router /api/persona [put]

// @Summary Reset persona
// @Description Reset persona to seed values from persona.md
// @Tags Persona
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/persona/reset [post]

// @Summary List topics
// @Description Get all forum topics with pagination
// @Tags Topics
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics [get]

// @Summary Create topic
// @Description Create a new forum topic
// @Tags Topics
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body topicCreateRequest false "Topic data"
// @Success 201 {object} map[string]interface{}
// @Router /api/topics [post]

// @Summary Get topic by ID
// @Description Get a single forum topic
// @Tags Topics
// @Security BearerAuth
// @Produce json
// @Param id path int true "Topic ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics/{id} [get]

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

// @Summary Delete topic
// @Description Delete a forum topic
// @Tags Topics
// @Security BearerAuth
// @Param id path int true "Topic ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics/{id} [delete]

// @Summary Close topic
// @Description Close a forum topic
// @Tags Topics
// @Security BearerAuth
// @Param id path int true "Topic ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics/{id}/close [post]

// @Summary Open topic
// @Description Reopen a closed forum topic
// @Tags Topics
// @Security BearerAuth
// @Param id path int true "Topic ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/topics/{id}/open [post]

// @Summary Get permissions
// @Description Get command permissions matrix
// @Tags Permissions
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/permissions [get]

// @Summary Update permission
// @Description Update allowed_role for a command
// @Tags Permissions
// @Security BearerAuth
// @Accept json
// @Param command path string true "Command name"
// @Param body body updatePermissionRequest false "New allowed_role"
// @Success 200 {object} map[string]interface{}
// @Router /api/permissions/{command} [put]

// @Summary Token usage stats
// @Description LLM token usage today by provider
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/tokens [get]

// @Summary Request stats
// @Description LLM request count today by type
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/requests [get]

// @Summary Forward stats
// @Description Message forward statistics
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/forwards [get]

// @Summary Message stats
// @Description Processed message statistics
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/messages [get]

// @Summary Provider stats
// @Description LLM provider performance stats
// @Tags Stats
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/stats/providers [get]

// @Summary Schedule snapshots
// @Description Schedule change snapshot history with pagination
// @Tags Schedule
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /api/schedule/snapshots [get]

// @Summary Schedule anomalies
// @Description Detected schedule anomalies with pagination
// @Tags Schedule
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /api/schedule/anomalies [get]
