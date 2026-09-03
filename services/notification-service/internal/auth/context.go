package auth

import "context"

type UserIdentity struct {
	UserID int64
	Role   string
}

type userIdentityContextKey struct{}

func WithUserIdentity(ctx context.Context, identity UserIdentity) context.Context {
	return context.WithValue(ctx, userIdentityContextKey{}, identity)
}

func UserIdentityFromContext(ctx context.Context) (UserIdentity, bool) {
	identity, ok := ctx.Value(userIdentityContextKey{}).(UserIdentity)
	return identity, ok && identity.UserID > 0
}
