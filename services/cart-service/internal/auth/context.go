package auth

import "context"

type UserIdentity struct {
	UserID int64
	Role   string
}

type ServiceIdentity struct {
	Authenticated bool
}

type userIdentityContextKey struct{}
type serviceIdentityContextKey struct{}

func WithUserIdentity(ctx context.Context, identity UserIdentity) context.Context {
	return context.WithValue(ctx, userIdentityContextKey{}, identity)
}

func UserIdentityFromContext(ctx context.Context) (UserIdentity, bool) {
	identity, ok := ctx.Value(userIdentityContextKey{}).(UserIdentity)
	return identity, ok && identity.UserID > 0
}

func WithServiceIdentity(ctx context.Context) context.Context {
	return context.WithValue(ctx, serviceIdentityContextKey{}, ServiceIdentity{Authenticated: true})
}

func ServiceIdentityFromContext(ctx context.Context) bool {
	identity, ok := ctx.Value(serviceIdentityContextKey{}).(ServiceIdentity)
	return ok && identity.Authenticated
}
