//go:build darwin

package vigil

import "context"

func list(ctx context.Context) ([]Process, error) {
	return nil, nil
}

func find(ctx context.Context, pid int) (*Process, error) {
	return nil, nil
}
