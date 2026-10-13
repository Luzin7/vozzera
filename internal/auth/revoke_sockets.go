package auth

import (
	"context"
	"log"
	"time"
)

const revokeSocketsTimeout = 2 * time.Second

func revokeSocketsDetached(ctx context.Context, revoke func(context.Context) error) {
	revokeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), revokeSocketsTimeout)
	defer cancel()

	if err := revoke(revokeCtx); err != nil {
		log.Printf("falha ao derrubar websockets depois de encerrar a sessão: %v", err)
	}
}
