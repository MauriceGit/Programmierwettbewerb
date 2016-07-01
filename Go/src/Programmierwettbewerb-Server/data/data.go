package data

import (
    . "Programmierwettbewerb-Server/vector"
    
    "fmt"
)

var EPSILON float32 = 0.00000001

func floatEquals(a, b float32) bool {
    return (a - b) < EPSILON && (b - a) < EPSILON
}

// -------------------------------------------------------------
// ValueBuffer
// -------------------------------------------------------------
 
type ValueBuffer interface {
    Append(value interface{})
}

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
// quadTreeEntry
// -------------------------------------------------------------

type quadTreeEntry struct {
    position    Vec2
    value       interface{}
}

// -------------------------------------------------------------
// QuadTreeInfo
// -------------------------------------------------------------

type QuadTreeInfo struct {
    EmptyNodeCount  int 
    InnerNodeCount  int 
    LeafNodeCount   int 
    EqualNodeCount  int 
}

func addQuadTreeInfos(i1 QuadTreeInfo, i2 QuadTreeInfo) QuadTreeInfo {
    return QuadTreeInfo{
        EmptyNodeCount: i1.EmptyNodeCount + i2.EmptyNodeCount,
        InnerNodeCount: i1.InnerNodeCount + i2.InnerNodeCount,
        LeafNodeCount: i1.LeafNodeCount + i2.LeafNodeCount,
        EqualNodeCount: i1.EqualNodeCount + i2.EqualNodeCount,
    }
}

func PrintQuadTreeInfo(info QuadTreeInfo) {
    fmt.Printf("EmptyNodes: %v\n", info.EmptyNodeCount)
    fmt.Printf("InnerNodes: %v\n", info.InnerNodeCount)
    fmt.Printf("LeafNodes: %v\n", info.LeafNodeCount)
    fmt.Printf("EqualNodes: %v\n", info.EqualNodeCount)
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

func (quadTree *QuadTree) Insert(position Vec2, value interface{}) {
    quadTree.root = quadTree.root.Insert(position, value)
}

func (quadTree *QuadTree) CountElements() int {
    return quadTree.root.CountElements()
}

func (quadTree *QuadTree) GetInfo() QuadTreeInfo {
    return quadTree.root.GetInfo()
}

func (quadTree *QuadTree) FindValuesInQuad(quad Quad, buffer ValueBuffer) {
    quadTree.root.FindValuesInQuad(quad, buffer)
}

func (quadTree *QuadTree) Print() {
    quadTree.root.Print("")
}

// -------------------------------------------------------------
// quadTreeNode
// -------------------------------------------------------------

type quadTreeNode interface { 
    Insert(position Vec2, value interface{}) quadTreeNode
    CountElements() int
    GetInfo() QuadTreeInfo
    FindValuesInQuad(quad Quad, buffer ValueBuffer)
    Print(indentation string)
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
    entry           quadTreeEntry
}

type equalNode struct {
    quad            Quad
    entry           quadTreeEntry
    next            quadTreeNode
}

func newEmptyNode(quad Quad) quadTreeNode {
    empty := new(emptyNode)
    empty.quad = quad
    return empty
}

func newLeafNode(quad Quad, entry quadTreeEntry) quadTreeNode {
    leaf := new(leafNode)
    leaf.quad = quad
    leaf.entry = entry
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

func newEqualNode(quad Quad, entry quadTreeEntry, next quadTreeNode) quadTreeNode {
    equal := new(equalNode)
    equal.quad = quad
    equal.entry = entry
    equal.next = next
    return equal
}

func (empty *emptyNode) CountElements() int {
    return 0
}

func (leaf *leafNode) CountElements() int {
    return 1
}

func (equal *equalNode) CountElements() int {
    return 1 + equal.next.CountElements()
}

func (inner *innerNode) CountElements() int {
    return inner.childLeftLower.CountElements() +
           inner.childRightLower.CountElements() +
           inner.childRightUpper.CountElements() +
           inner.childLeftUpper.CountElements();
}

func (empty *emptyNode) GetInfo() QuadTreeInfo {
    return QuadTreeInfo{ EmptyNodeCount: 1 }
}

func (leaf *leafNode) GetInfo() QuadTreeInfo {
    return QuadTreeInfo{ LeafNodeCount: 1 }
}

func (equal *equalNode) GetInfo() QuadTreeInfo {
    return addQuadTreeInfos(QuadTreeInfo{ EqualNodeCount: 1 }, equal.next.GetInfo())
}

func (inner *innerNode) GetInfo() QuadTreeInfo {    
    lower := addQuadTreeInfos(inner.childLeftLower.GetInfo(), inner.childRightLower.GetInfo())
    upper := addQuadTreeInfos(inner.childLeftUpper.GetInfo(), inner.childRightUpper.GetInfo())
    children := addQuadTreeInfos(lower, upper)
    return addQuadTreeInfos(QuadTreeInfo{ InnerNodeCount: 1 }, children)
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

func (inner *innerNode) Insert(position Vec2, value interface{}) quadTreeNode {
    halfSize := inner.quad.Size/2
    if position.X < inner.quad.Origin.X + halfSize {
        if position.Y < inner.quad.Origin.Y + halfSize {
            inner.childLeftLower = inner.childLeftLower.Insert(position, value)
        } else {
            inner.childLeftUpper = inner.childLeftUpper.Insert(position, value)
        }
    } else {
        if position.Y < inner.quad.Origin.Y + halfSize {
            inner.childRightLower = inner.childRightLower.Insert(position, value)
        } else {
            inner.childRightUpper = inner.childRightUpper.Insert(position, value)
        }
    }
    return inner
}

func (empty *emptyNode) Insert(position Vec2, value interface{}) quadTreeNode {
    return newLeafNode(empty.quad, quadTreeEntry{ position, value })
}

func (leaf *leafNode) Insert(position Vec2, value interface{}) quadTreeNode {
    if floatEquals(leaf.entry.position.X, position.X) && 
       floatEquals(leaf.entry.position.Y, position.Y) {
        return newEqualNode(leaf.quad, quadTreeEntry{ position, value }, leaf)
    }
    
    otherEntry := leaf.entry
    node := newInnerNode(leaf.quad)
    node = node.Insert(position, value)
    node = node.Insert(otherEntry.position, otherEntry.value)
    return node
}

func (equal *equalNode) Insert(position Vec2, value interface{}) quadTreeNode {
    return newEqualNode(equal.quad, quadTreeEntry{ position, value }, equal)
}


func intervalsOverlap(x1 float32, x2 float32, y1 float32, y2 float32) bool {
    return x1 <= y2 && y1 <= x2
}

func quadsOverlap(q1 Quad, q2 Quad) bool {
    horizontal := intervalsOverlap(q1.Origin.X, q1.Origin.X + q1.Size, q2.Origin.X, q2.Origin.X + q2.Size)
    vertical   := intervalsOverlap(q1.Origin.Y, q1.Origin.Y + q1.Size, q2.Origin.Y, q2.Origin.Y + q2.Size)
    return horizontal && vertical
}

func TEST() {
    /*
    q1 := NewQuad(Vec2{0,0}, 2)
    q2 := NewQuad(Vec2{-3,-3}, 2)
    
    if quadsOverlap(q1, q2) {
        fmt.Printf("Overlap\n")
    }
    */
}


func (empty *emptyNode) Print(indentation string) {
    fmt.Printf("%sEmpty\n", indentation)
}

func (leaf *leafNode) Print(indentation string) {
    fmt.Printf("%sLeaf (%v)\n", indentation, leaf.quad) 
}

func (inner *innerNode) Print(indentation string) {
    fmt.Printf("%sInner (%v)\n", indentation, inner.quad)
    newIndentation := indentation + " "
    inner.childLeftLower.Print(newIndentation)
    inner.childRightLower.Print(newIndentation)
    inner.childRightUpper.Print(newIndentation)
    inner.childLeftUpper.Print(newIndentation)
}

func (equal *equalNode) Print(indentation string) {
    fmt.Printf("%sEqual (%v)\n", indentation, equal.quad)
    equal.next.Print(indentation + " ")
}


func (empty *emptyNode) FindValuesInQuad(quad Quad, buffer ValueBuffer) {
    // Nothing has to happen here
}

func (leaf *leafNode) FindValuesInQuad(quad Quad, buffer ValueBuffer) {
    if quadsOverlap(quad, leaf.quad) {
        buffer.Append(leaf.entry.value)
    }
}

func (inner *innerNode) FindValuesInQuad(quad Quad, buffer ValueBuffer) {
    if quadsOverlap(quad, inner.quad) {
        inner.childLeftLower.FindValuesInQuad(quad, buffer)
        inner.childRightLower.FindValuesInQuad(quad, buffer)
        inner.childRightUpper.FindValuesInQuad(quad, buffer)
        inner.childLeftUpper.FindValuesInQuad(quad, buffer)
    }
}

func (equal *equalNode) FindValuesInQuad(quad Quad, buffer ValueBuffer) {
    if quadsOverlap(quad, equal.quad) {
        buffer.Append(equal.entry.value)
        equal.next.FindValuesInQuad(quad, buffer)
    }
}
