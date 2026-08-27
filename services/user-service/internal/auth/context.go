package auth

import (
	"context"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
)

type identityContextKey struct{}

// WithIdentity attaches an already verified identity to the request context.
// The gRPC authentication interceptor will call this function later.
func WithIdentity(ctx context.Context, identity domain.Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (domain.Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(domain.Identity)
	return identity, ok
}
