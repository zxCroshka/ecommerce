package user

type UpdateEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
}

type UpdateNameRequest struct {
	NewName string `json:"new_name" binding:"required,min=2,max=100"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}
