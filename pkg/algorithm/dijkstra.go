// This code is copied from https://blog.amedama.jp/entry/2015/10/20/202245
// Thanks so much!

package algorithm

import (
	"errors"
	"fmt"
	"slices"
)

// ノード
type Node struct {
	name  string  // ノード名
	edges []*Edge // 次に移動できるエッジ
	done  bool    // 処理済みかを表すフラグ
	cost  int     // このノードにたどり着くのに必要だったコスト
	prev  *Node   // このノードにたどりつくのに使われたノード
	via   *Edge   // the edge used to reach this node
}

func NewNode(name string) *Node {
	node := &Node{name, []*Edge{}, false, -1, nil, nil}
	return node
}

// ノードに次の接続先を示したエッジを追加する
func (n *Node) AddEdge(edge *Edge) {
	n.edges = append(n.edges, edge)
}

func (n *Node) GetName() string {
	return n.name
}

// エッジ
type Edge struct {
	name string
	next *Node // 次に移動できるノード
	cost int   // 移動にかかるコスト
}

func NewEdge(name string, next *Node, cost int) *Edge {
	edge := &Edge{name, next, cost}
	return edge
}

func (e *Edge) GetName() string {
	return e.name
}

// 有向グラフ
type DirectedGraph struct {
	nodes map[string]*Node
}

func NewDirectedGraph() *DirectedGraph {
	return &DirectedGraph{
		map[string]*Node{}}
}

func (self *DirectedGraph) Print() {
	for k, v := range self.nodes {
		fmt.Printf("k=%v v=%v\n", k, v)
	}
}

// グラフの要素を追加する (接続元ノード名、接続先ノード名、Edge Name, 移動にかかるコスト)
func (self *DirectedGraph) Add(src, dst, edgeName string, cost int) {
	// ノードが既にある場合は追加しない
	srcNode, ok := self.nodes[src]
	if !ok {
		srcNode = NewNode(src)
		self.nodes[src] = srcNode
	}

	dstNode, ok := self.nodes[dst]
	if !ok {
		dstNode = NewNode(dst)
		self.nodes[dst] = dstNode
	}

	// ノードをエッジでつなぐ
	edge := NewEdge(edgeName, dstNode, cost)
	srcNode.AddEdge(edge)
}

// スタートとゴールを指定して最短経路を求める
func (self *DirectedGraph) ShortestPath(start string, goal string) (ret []*Node, via []*Edge, err error) {
	return self.ShortestPathWithMultipleGoals(start, []string{goal})
}

// スタートとゴールを指定して最短経路を求める
func (self *DirectedGraph) ShortestPathWithMultipleGoals(start string, goals []string) (ret []*Node, via []*Edge, err error) {
	// 名前からスタート地点のノードを取得する
	startNode := self.nodes[start]

	// スタートのコストを 0 に設定することで処理対象にする
	startNode.cost = 0

	goal := ""
	for {
		// 次の処理対象のノードを取得する
		node, err := self.nextNode()

		// 次に処理するノードが見つからなければ終了
		if err != nil {
			return nil, nil, errors.New("Goal not found")
		}

		// ゴールまで到達した
		if slices.Contains(goals, node.name) {
			goal = node.name
			break
		}

		// 取得したノードを処理する
		self.calc(node)
	}

	// ゴールから逆順にスタートまでノードをたどっていく
	n := self.nodes[goal]
	ret_rev := make([]*Node, 0)
	viaEdgesRev := make([]*Edge, 0)
	for {
		ret_rev = append(ret_rev, n)
		if n.name == start {
			break
		}
		viaEdgesRev = append(viaEdgesRev, n.via)
		n = n.prev
	}

	for i := range ret_rev {
		ret = append(ret, ret_rev[len(ret_rev)-i-1])
	}

	for i := range viaEdgesRev {
		via = append(via, viaEdgesRev[len(viaEdgesRev)-i-1])
	}

	// Reset all nodes
	for i := range self.nodes {
		self.nodes[i].done = false
		self.nodes[i].cost = -1
		self.nodes[i].prev = nil
		self.nodes[i].via = nil
	}

	return ret, via, nil
}

// つながっているノードのコストを計算する
func (self *DirectedGraph) calc(node *Node) {
	// ノードにつながっているエッジを取得する
	for i, edge := range node.edges {
		nextNode := edge.next

		// 既に処理済みのノードならスキップする
		if nextNode.done {
			continue
		}

		// このノードに到達するのに必要なコストを計算する
		cost := node.cost + edge.cost
		if nextNode.cost == -1 || cost < nextNode.cost {
			// 既に見つかっている経路よりもコストが小さければ処理中のノードを遷移元として記録する
			nextNode.cost = cost
			nextNode.prev = node
			nextNode.via = node.edges[i]
		}
	}

	// つながっているノードのコスト計算がおわったらこのノードは処理済みをマークする
	node.done = true
}

func (self *DirectedGraph) nextNode() (next *Node, err error) {
	// グラフに含まれるノードを線形探索する
	for _, node := range self.nodes {

		// 処理済みのノードは対象外
		if node.done {
			continue
		}

		// コストが初期値 (-1) になっているノードはまだそのノードまでの最短経路が判明していないので処理できない
		if node.cost == -1 {
			continue
		}

		// 最初に見つかったものは問答無用で次の処理対象の候補になる
		if next == nil {
			next = node
		}

		// 既に見つかったノードよりもコストの小さいものがあればそちらを先に処理しなければいけない
		if next.cost > node.cost {
			next = node
		}
	}

	// 次の処理対象となるノードが見つからなかったときはエラー
	if next == nil {
		return nil, errors.New("Untreated node not found")
	}

	return
}
