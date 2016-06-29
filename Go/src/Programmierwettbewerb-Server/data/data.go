package data

import (
    . "Programmierwettbewerb-Server/vector"
)

type OctreeValue struct {
    position    Vec2
    size        float32
    id          int
}

type Octree interface { 
    Insert(value OctreeValue) Octree
}

type EmptyNode struct { }
var emptyNode *EmptyNode

type InnerNode struct {
    origin          Vec2
    size            float32
    childLeftLower  Octree
    childRightLower Octree
    childRightUpper Octree
    childLeftUpper  Octree
}

type LeafNode struct {
    value           OctreeValue
}

func NewOctree() Octree {
    if emptyNode == nil {
        emptyNode = new(EmptyNode)
    }
    return emptyNode
}

func newLeaf(value OctreeValue) Octree {
    return new(LeafNode)
}

func newInnerNode() Octree {
    return new(InnerNode)
}

func containsPoint(innerNode *InnerNode, point Vec2) bool {
    return point.X > innerNode.origin.X &&
           point.X < innerNode.origin.X + innerNode.size &&
           point.Y > innerNode.origin.Y &&
           point.Y < innerNode.origin.Y + innerNode.size;
}

func leftLowerSpace(innerNode *InnerNode) (Vec2, float32) {
    return innerNode.origin, innerNode.size/2
}

func rightLowerSpace(innerNode *InnerNode) (Vec2, float32) {
    halfSize := innerNode.size/2
    origin := Vec2{ X:innerNode.origin.X + halfSize, Y:innerNode.origin.Y }
    return origin, halfSize
}

func rightUpperSpace(innerNode *InnerNode) (Vec2, float32) {
    halfSize := innerNode.size/2
    origin := Vec2{ X:innerNode.origin.X + halfSize, Y:innerNode.origin.Y + halfSize }
    return origin, halfSize
}

func leftUpperSpace(innerNode *InnerNode) (Vec2, float32) {
    halfSize := innerNode.size/2
    origin := Vec2{ X:innerNode.origin.X, Y:innerNode.origin.Y + halfSize }
    return origin, halfSize
}

func (innerNode *InnerNode) Insert(value OctreeValue) Octree {
    halfSize := innerNode.size/2
    if value.position.X < innerNode.origin.X + halfSize {
        if value.position.Y < innerNode.origin.Y + halfSize {
            innerNode.childLeftLower = innerNode.childLeftLower.Insert(value)
        } else {
            innerNode.childLeftUpper = innerNode.childLeftUpper.Insert(value)
        }
    } else {
        if value.position.Y < innerNode.origin.Y + halfSize {
            innerNode.childRightLower = innerNode.childRightLower.Insert(value)
        } else {
            innerNode.childRightUpper = innerNode.childRightUpper.Insert(value)
        }
    }
    return innerNode
}

func (emptyNode *EmptyNode) Insert(value OctreeValue) Octree {
    return newLeaf(value)
}

func (leafNode *LeafNode) Insert(value OctreeValue) Octree {
    otherValue := leafNode.value
    node := newInnerNode()
    node = node.Insert(value)
    node = node.Insert(otherValue)
    return node
}

