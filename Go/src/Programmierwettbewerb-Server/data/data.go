package data

import (
    . "Programmierwettbewerb-Server/vector"
)

// -------------------------------------------------------------
// Quad
// -------------------------------------------------------------

type Quad struct {
    Origin  Vec2
    Size    float32
}

func NewQuad(origin Vec2, size float32) Quad {
    return Quad{ Origin:origin, Size:size }
}

// -------------------------------------------------------------
// QuadTreeValue
// -------------------------------------------------------------

type QuadTreeValue interface {
    GetPosition() Vec2
}

// -------------------------------------------------------------
// QuadTree
// -------------------------------------------------------------

type QuadTree struct {
    root    quadTreeNode
}

func NewQuadTree(quad Quad) QuadTree {
    return QuadTree{ root:newEmptyNode(quad) }
}

func (quadTree *QuadTree) Insert(value QuadTreeValue) {
    quadTree.root = quadTree.root.Insert(value)
}

func (quadTree *QuadTree) CountElements() int {
    return quadTree.root.CountElements()
}

// -------------------------------------------------------------
// quadTreeNode
// -------------------------------------------------------------

type quadTreeNode interface { 
    Insert(value QuadTreeValue) quadTreeNode
    CountElements() int
}


type emptyNode struct { 
    quad            Quad
}

type innerNode struct {
    quad            Quad
    childLeftLower  quadTreeNode
    childRightLower quadTreeNode
    childRightUpper quadTreeNode
    childLeftUpper  quadTreeNode
}

type leafNode struct {
    quad            Quad
    value           QuadTreeValue
}

func (empty *emptyNode) CountElements() int {
    return 0
}

func (leaf *leafNode) CountElements() int {
    return 1
}

func (inner *innerNode) CountElements() int {
    return inner.childLeftLower.CountElements() +
           inner.childRightLower.CountElements() +
           inner.childRightUpper.CountElements() +
           inner.childLeftUpper.CountElements();
}

func newEmptyNode(quad Quad) quadTreeNode {
    empty := new(emptyNode)
    empty.quad = quad
    return empty
}

func newLeafNode(quad Quad, value QuadTreeValue) quadTreeNode {
    leaf := new(leafNode)
    leaf.quad = quad
    leaf.value = value
    return leaf
}

func newInnerNode(quad Quad) quadTreeNode {
    inner := new(innerNode)
    inner.quad = quad
    inner.childLeftLower = newEmptyNode(leftLowerSpace(inner))
    inner.childRightLower = newEmptyNode(rightLowerSpace(inner))
    inner.childRightUpper = newEmptyNode(rightUpperSpace(inner))
    inner.childLeftUpper = newEmptyNode(leftUpperSpace(inner))
    return inner
}

func quadContainsPoint(quad Quad, point Vec2) bool {
    return point.X > quad.Origin.X &&
           point.X < quad.Origin.X + quad.Size &&
           point.Y > quad.Origin.Y &&
           point.Y < quad.Origin.Y + quad.Size;
}

func innerNodeContainsPoint(inner *innerNode, point Vec2) bool {
    return quadContainsPoint(inner.quad, point);
}

func leftLowerSpace(inner *innerNode) Quad {
    halfSize := inner.quad.Size/2
    return NewQuad(inner.quad.Origin, halfSize)
}

func rightLowerSpace(inner *innerNode) Quad {
    halfSize := inner.quad.Size/2
    origin := Vec2{ X:inner.quad.Origin.X + halfSize, Y:inner.quad.Origin.Y }
    return NewQuad(origin, halfSize)
}

func rightUpperSpace(inner *innerNode) Quad {
    halfSize := inner.quad.Size/2
    origin := Vec2{ X:inner.quad.Origin.X + halfSize, Y:inner.quad.Origin.Y + halfSize }
    return NewQuad(origin, halfSize)
}

func leftUpperSpace(inner *innerNode) Quad {
    halfSize := inner.quad.Size/2
    origin := Vec2{ X:inner.quad.Origin.X, Y:inner.quad.Origin.Y + halfSize }
    return NewQuad(origin, halfSize)
}

func (inner *innerNode) Insert(value QuadTreeValue) quadTreeNode {
    halfSize := inner.quad.Size/2
    if value.GetPosition().X < inner.quad.Origin.X + halfSize {
        if value.GetPosition().Y < inner.quad.Origin.Y + halfSize {
            inner.childLeftLower = inner.childLeftLower.Insert(value)
        } else {
            inner.childLeftUpper = inner.childLeftUpper.Insert(value)
        }
    } else {
        if value.GetPosition().Y < inner.quad.Origin.Y + halfSize {
            inner.childRightLower = inner.childRightLower.Insert(value)
        } else {
            inner.childRightUpper = inner.childRightUpper.Insert(value)
        }
    }
    return inner
}

func (empty *emptyNode) Insert(value QuadTreeValue) quadTreeNode {
    return newLeafNode(empty.quad, value)
}

func (leaf *leafNode) Insert(value QuadTreeValue) quadTreeNode {
    otherValue := leaf.value
    node := newInnerNode(leaf.quad)
    node = node.Insert(value)
    node = node.Insert(otherValue)
    return node
}
