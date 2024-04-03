package algorithm_test

import (
	"testing"

	"github.com/naoki9911/fuse-diff-containerd/pkg/algorithm"
	"github.com/stretchr/testify/assert"
)

func TestDijkstra(t *testing.T) {
	g := algorithm.NewDirectedGraph()

	g.Add("1.23.1", "1.23.2", "hoge1", 1)
	g.Add("1.23.2", "1.23.3", "hoge2", 1)
	g.Add("1.23.3", "1.23.4", "hoge3", 1)
	g.Add("1.23.1", "1.23.3", "hoge4", 1)

	path, via, err := g.ShortestPath("1.23.1", "1.23.4")
	assert.Equal(t, nil, err)
	assert.Equal(t, 3, len(path))
	assert.Equal(t, "1.23.1", path[0].GetName())
	assert.Equal(t, "1.23.3", path[1].GetName())
	assert.Equal(t, "1.23.4", path[2].GetName())

	assert.Equal(t, nil, err)
	assert.Equal(t, 2, len(via))
	assert.Equal(t, "hoge4", via[0].GetName())
	assert.Equal(t, "hoge3", via[1].GetName())
}
