package auth

import "context"

type UserIdentity struct {
	UserID int64
	Role   string
}

type userIdentityKey struct{}

func WithUserIdentity(ctx context.Context, identity UserIdentity) context.Context {
	return context.WithValue(ctx, userIdentityKey{}, identity)
}

func UserIdentityFromContext(ctx context.Context) (UserIdentity, bool) {
	identity, ok := ctx.Value(userIdentityKey{}).(UserIdentity)
	return identity, ok && identity.UserID > 0
}
