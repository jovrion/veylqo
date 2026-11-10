package link

import "context"

// These stubs exist so a signature drift fails `go test` in this package.
// They are not the memory adapter (VYQ-24).

type repositoryStub struct{}

func (repositoryStub) Add(context.Context, Link) error { return nil }

func (repositoryStub) Get(context.Context, Code) (Link, error) { return Link{}, nil }

func (repositoryStub) GetByDestination(context.Context, Destination) (Link, error) {
	return Link{}, nil
}

func (repositoryStub) Update(context.Context, Code, func(*Link) error) error { return nil }

type clickRepositoryStub struct{}

func (clickRepositoryStub) Add(context.Context, Code, int64) error { return nil }

func (clickRepositoryStub) Count(context.Context, Code) (int64, error) { return 0, nil }

var (
	_ Repository      = repositoryStub{}
	_ ClickRepository = clickRepositoryStub{}
)
