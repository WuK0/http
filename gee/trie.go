package gee

import "strings"

type node struct {
	pattern string
	part string
	isWild bool
	children []*node
}

func (n *node) matchChild(part string) *node {
	for _, child := range n.children {
		if child.part == part || child.isWild {
			return child
		}
	}
	return nil
}
func (n *node) matchChildren(part string) []*node {
	res := make([]*node, 0)
	for _, child := range n.children {
		if child.part == part || child.isWild {
			res = append(res, child)
		}
	}
	return res
}

func (n *node) insert(pattern string, parts []string, height int)  {
	if len(parts) == height {
		n.pattern = pattern
		return
	}
	part := parts[height]
	child := n.matchChild(part)
	if child == nil {
		// 没有节点则新增
		child = &node{
			part: part,
			isWild: part[0] == ':' || part[0] == '*',
		}
		n.children = append(n.children, child)
	}
	child.insert(pattern, parts, height + 1)
}

func (n *node) search(parts []string, height int) *node {
	if height == len(parts) || strings.HasPrefix(n.part, "*"){
		if n.pattern == "" {
			return nil
		}
		return n
	}
	part := parts[height]
	for _, child := range n.matchChildren(part) {
		result := child.search(parts, height + 1)
		if result != nil {
			return result
		}
	}
	return nil
}