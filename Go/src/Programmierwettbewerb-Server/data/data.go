package data

import (
    . "Programmierwettbewerb-Server/vector"
)

type OctreeValue interface {
    GetPosition() Vec2
    GetSize() float32
}

type Octree interface { 
    Insert(value OctreeValue) Octree
}

type emptyNode struct { }
var empty *emptyNode

type innerNode struct {
    origin          Vec2
    size            float32
    childLeftLower  Octree
    childRightLower Octree
    childRightUpper Octree
    childLeftUpper  Octree
}

type leafNode struct {
    value           OctreeValue
}

func NewOctree() Octree {
    if empty == nil {
        empty = new(emptyNode)
    }
    return empty
}

func newLeafNode(value OctreeValue) Octree {
    return new(leafNode)
}

func newInnerNode() Octree {
    return new(innerNode)
}

func containsPoint(inner *innerNode, point Vec2) bool {
    return point.X > inner.origin.X &&
           point.X < inner.origin.X + inner.size &&
           point.Y > inner.origin.Y &&
           point.Y < inner.origin.Y + inner.size;
}

func leftLowerSpace(inner *innerNode) (Vec2, float32) {
    return inner.origin, inner.size/2
}

func rightLowerSpace(inner *innerNode) (Vec2, float32) {
    halfSize := inner.size/2
    origin := Vec2{ X:inner.origin.X + halfSize, Y:inner.origin.Y }
    return origin, halfSize
}

func rightUpperSpace(inner *innerNode) (Vec2, float32) {
    halfSize := inner.size/2
    origin := Vec2{ X:inner.origin.X + halfSize, Y:inner.origin.Y + halfSize }
    return origin, halfSize
}

func leftUpperSpace(inner *innerNode) (Vec2, float32) {
    halfSize := inner.size/2
    origin := Vec2{ X:inner.origin.X, Y:inner.origin.Y + halfSize }
    return origin, halfSize
}

func (inner *innerNode) Insert(value OctreeValue) Octree {
    halfSize := inner.size/2
    if value.GetPosition().X < inner.origin.X + halfSize {
        if value.GetPosition().Y < inner.origin.Y + halfSize {
            inner.childLeftLower = inner.childLeftLower.Insert(value)
        } else {
            inner.childLeftUpper = inner.childLeftUpper.Insert(value)
        }
    } else {
        if value.GetPosition().Y < inner.origin.Y + halfSize {
            inner.childRightLower = inner.childRightLower.Insert(value)
        } else {
            inner.childRightUpper = inner.childRightUpper.Insert(value)
        }
    }
    return inner
}

func (empty *emptyNode) Insert(value OctreeValue) Octree {
    return newLeafNode(value)
}

func (leaf *leafNode) Insert(value OctreeValue) Octree {
    otherValue := leaf.value
    node := newInnerNode()
    node = node.Insert(value)
    node = node.Insert(otherValue)
    return node
}

