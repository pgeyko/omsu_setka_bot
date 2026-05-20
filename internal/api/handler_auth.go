package api

// @Summary Get JWT token
// @Description Authenticate with admin_secret and receive a JWT token
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body object false "admin_secret"
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/auth/token [post]
func _auth_stub() {}
