package data

import (
    . "Programmierwettbewerb-Server/vector"
    . "Programmierwettbewerb-Server/shared"
    
    "fmt"
    "encoding/json"
)

var EPSILON float32 = 0.00000001

func floatEquals(a, b float32) bool {
    return (a - b) < EPSILON && (b - a) < EPSILON
}

////////////////////////////////////////////////////////////////////////
//
// ValueBuffer
//
////////////////////////////////////////////////////////////////////////
 
type ValueBuffer interface {
    Append(value interface{})
}

////////////////////////////////////////////////////////////////////////
//
// Quad
//
////////////////////////////////////////////////////////////////////////

type Quad struct {
    Origin  Vec2
    Size    float32
}

func NewQuad(origin Vec2, size float32) Quad {
    return Quad{ Origin:origin, Size:size }
}

////////////////////////////////////////////////////////////////////////
//
// quadTreeEntry
//
////////////////////////////////////////////////////////////////////////

type quadTreeEntry struct {
    position    Vec2
    value       interface{}
}

func (entry *quadTreeEntry) MarshalJSON() ([]byte, error) {
    return json.Marshal(&struct {
        Position    Vec2            `json:"pos"`
    }{
        Position:   entry.position,
    })
}

////////////////////////////////////////////////////////////////////////
//
// QuadTreeInfo
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// QuadTree
//
////////////////////////////////////////////////////////////////////////

type QuadTree struct {
    root        quadTreeNode
    allocator   *Allocator
}

func NewQuadTree(quad Quad, allocator *Allocator) QuadTree {
    return QuadTree{ 
        root:       newEmptyNode(allocator, quad), 
        allocator:  allocator,
    }
}

func (quadTree *QuadTree) Insert(position Vec2, value interface{}) {
    quadTree.root = quadTree.root.Insert(quadTree.allocator, position, value)
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

func (quadTree *QuadTree) ToJson() ([]byte, error) {
    return json.MarshalIndent(quadTree.root, "", "  ")
}

////////////////////////////////////////////////////////////////////////
//
// quadTreeNode
//
////////////////////////////////////////////////////////////////////////

type quadTreeNode interface { 
    Insert(allocator *Allocator, position Vec2, value interface{}) quadTreeNode
    CountElements() int
    GetInfo() QuadTreeInfo
    FindValuesInQuad(quad Quad, buffer ValueBuffer)
    Print(indentation string)
}

////////////////////////////////////////////////////////////////////////
//
// NODES
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// Allocator
//
////////////////////////////////////////////////////////////////////////

type Allocator struct {
    emptyNodeBatches        []EmptyNodeAllocatorBatch
    
    equalNodes              []equalNode
    numUsedEqualNodes       int
            
    innerNodes              []innerNode
    numUsedInnerNodes       int
            
    leafNodes               []leafNode
    numUsedLeafNodes        int
    
    LimitWasHit             bool
}

type EmptyNodeAllocatorBatch struct {
    nodes           []emptyNode
    numUsedNodes    int
}

func NewEmptyNodeAllocatorBatch(numEmptyNodesPerBatch int) EmptyNodeAllocatorBatch {
    return EmptyNodeAllocatorBatch{
        nodes:          make([]emptyNode, numEmptyNodesPerBatch, numEmptyNodesPerBatch),
        numUsedNodes:   0,
    }
}

func NewAllocator(numEmptyNodesPerBatch, numEqualNodes, numInnerNodes, numLeafNodes int) Allocator {
    allocator := Allocator{
        emptyNodeBatches:       make([]EmptyNodeAllocatorBatch, 0, 2),
        
        equalNodes:             make([]equalNode, numEqualNodes, numEqualNodes),
        numUsedEqualNodes:      0,
        
        innerNodes:             make([]innerNode, numInnerNodes, numInnerNodes),
        numUsedInnerNodes:      0,
        
        leafNodes:              make([]leafNode, numLeafNodes, numLeafNodes),
        numUsedLeafNodes:       0,
    }
    
    allocator.emptyNodeBatches = append(allocator.emptyNodeBatches, NewEmptyNodeAllocatorBatch(numEmptyNodesPerBatch))
    
    return allocator
}

func allocFromBatch(batch *EmptyNodeAllocatorBatch) (*emptyNode, bool) {
    if batch.numUsedNodes >= cap(batch.nodes) {
        return nil, false
    }
    defer func() { batch.numUsedNodes += 1 }()
    return &batch.nodes[batch.numUsedNodes], true
}

func (allocator *Allocator) allocEmptyNode() *emptyNode {
    lastBatch := func() *EmptyNodeAllocatorBatch {
        return &allocator.emptyNodeBatches[len(allocator.emptyNodeBatches) - 1]
    }
    
    batch  := lastBatch()
    if node, success := allocFromBatch(batch); success {
        return node
    }

    allocator.emptyNodeBatches = append(allocator.emptyNodeBatches, NewEmptyNodeAllocatorBatch(cap(batch.nodes)))
    node, _ := allocFromBatch(lastBatch())
    return node
}

func (allocator *Allocator) allocEqualNode() (*equalNode, bool) {
    if allocator.numUsedEqualNodes + 1 > cap(allocator.equalNodes) {
        allocator.LimitWasHit = true
        return nil, false
    }
    defer func() { allocator.numUsedEqualNodes += 1 }()
    return &allocator.equalNodes[allocator.numUsedEqualNodes], true
}

func (allocator *Allocator) allocInnerNode() (*innerNode, bool) {
    if allocator.numUsedInnerNodes + 1 > cap(allocator.innerNodes) {
        allocator.LimitWasHit = true
        return nil, false
    }
    defer func() { allocator.numUsedInnerNodes += 1 }()
    return &allocator.innerNodes[allocator.numUsedInnerNodes], true
}

func (allocator *Allocator) allocLeafNode() (*leafNode, bool) {
    if allocator.numUsedLeafNodes + 1 > cap(allocator.leafNodes) {
        allocator.LimitWasHit = true
        return nil, false
    }
    defer func() { allocator.numUsedLeafNodes += 1 }()
    return &allocator.leafNodes[allocator.numUsedLeafNodes], true
}

func (allocator *Allocator) Report() {
    numEmptyNodes := 0
    for _, batch := range allocator.emptyNodeBatches {
        numEmptyNodes += batch.numUsedNodes
    }
    Logf(LtDebug, "NumEmptyNodes: %v\n", numEmptyNodes)
    Logf(LtDebug, "NumEqualNodes: %v\n", allocator.numUsedEqualNodes)
    Logf(LtDebug, "NumInnerNodes: %v\n", allocator.numUsedInnerNodes)
    Logf(LtDebug, "NumLeafNodes: %v\n", allocator.numUsedLeafNodes)
}

////////////////////////////////////////////////////////////////////////
//
// CONTSTRUCTORS
//
////////////////////////////////////////////////////////////////////////

func newEmptyNode(allocator *Allocator, quad Quad) quadTreeNode {
    empty := allocator.allocEmptyNode()
    empty.quad = quad
    return empty
}

func newLeafNode(allocator *Allocator, quad Quad, entry quadTreeEntry) (quadTreeNode, bool) {
    leaf, success := allocator.allocLeafNode()
    if !success {
        return allocator.allocEmptyNode(), false
    }
    leaf.quad = quad
    leaf.entry = entry
    return leaf, true
}

func newInnerNode(allocator *Allocator, quad Quad) (quadTreeNode, bool) {
    inner, success := allocator.allocInnerNode()
    if !success {
        return allocator.allocEmptyNode(), false
    }
    inner.quad = quad
    inner.childLeftLower = newEmptyNode(allocator, leftLowerSpace(inner))
    inner.childRightLower = newEmptyNode(allocator, rightLowerSpace(inner))
    inner.childRightUpper = newEmptyNode(allocator, rightUpperSpace(inner))
    inner.childLeftUpper = newEmptyNode(allocator, leftUpperSpace(inner))
    return inner, true
}

func newEqualNode(allocator *Allocator, quad Quad, entry quadTreeEntry, next quadTreeNode) (quadTreeNode, bool) {
    equal, success := allocator.allocEqualNode()
    if !success {
        return allocator.allocEmptyNode(), false
    }    
    equal.quad = quad
    equal.entry = entry
    equal.next = next
    return equal, true
}

////////////////////////////////////////////////////////////////////////
//
// CountElements
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// GetInfo
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// quadContainsPoint
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// Insert
//
////////////////////////////////////////////////////////////////////////

func (inner *innerNode) Insert(allocator *Allocator, position Vec2, value interface{}) quadTreeNode {
    halfSize := inner.quad.Size/2
    if position.X < inner.quad.Origin.X + halfSize {
        if position.Y < inner.quad.Origin.Y + halfSize {
            inner.childLeftLower = inner.childLeftLower.Insert(allocator, position, value)
        } else {
            inner.childLeftUpper = inner.childLeftUpper.Insert(allocator, position, value)
        }
    } else {
        if position.Y < inner.quad.Origin.Y + halfSize {
            inner.childRightLower = inner.childRightLower.Insert(allocator, position, value)
        } else {
            inner.childRightUpper = inner.childRightUpper.Insert(allocator, position, value)
        }
    }
    return inner
}

func (empty *emptyNode) Insert(allocator *Allocator, position Vec2, value interface{}) quadTreeNode {
    leaf, _ := newLeafNode(allocator, empty.quad, quadTreeEntry{ position, value })
    return leaf
}

func (leaf *leafNode) Insert(allocator *Allocator, position Vec2, value interface{}) quadTreeNode {
    if floatEquals(leaf.entry.position.X, position.X) && 
       floatEquals(leaf.entry.position.Y, position.Y) {
        equal, _ := newEqualNode(allocator, leaf.quad, quadTreeEntry{ position, value }, leaf)
        return equal
    }
    
    otherEntry := leaf.entry
    node, success := newInnerNode(allocator, leaf.quad)
    if success {
        node = node.Insert(allocator, position, value)
        node = node.Insert(allocator, otherEntry.position, otherEntry.value)
    }
    return node
}

func (equal *equalNode) Insert(allocator *Allocator, position Vec2, value interface{}) quadTreeNode {
    head, _ := newEqualNode(allocator, equal.quad, quadTreeEntry{ position, value }, equal)
    return head
}


func intervalsOverlap(x1 float32, x2 float32, y1 float32, y2 float32) bool {
    return x1 <= y2 && y1 <= x2
}

func quadsOverlap(q1 Quad, q2 Quad) bool {
    horizontal := intervalsOverlap(q1.Origin.X, q1.Origin.X + q1.Size, q2.Origin.X, q2.Origin.X + q2.Size)
    vertical   := intervalsOverlap(q1.Origin.Y, q1.Origin.Y + q1.Size, q2.Origin.Y, q2.Origin.Y + q2.Size)
    return horizontal && vertical
}

////////////////////////////////////////////////////////////////////////
//
// Print
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// FindValuesInQuad
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// ToJson
//
////////////////////////////////////////////////////////////////////////

func (empty *emptyNode) MarshalJSON() ([]byte, error) {
    return json.Marshal(empty.quad)
}

func (leaf *leafNode) MarshalJSON() ([]byte, error) {
    return json.Marshal(&struct {
        Quad                Quad            `json:"quad"`
        Entry               quadTreeEntry   `json:"entry"`
    }{
        Quad:               leaf.quad,
        Entry:              leaf.entry,
    })
}

func (inner *innerNode) MarshalJSON() ([]byte, error) {
    return json.Marshal(&struct {
        Quad                Quad            `json:"quad"`
        ChildLeftLower      quadTreeNode    `json:"leftlower"`
        ChildRightLower     quadTreeNode    `json:"rightlower"`
        ChildRightUpper     quadTreeNode    `json:"rightupper"`
        ChildLeftUpper      quadTreeNode    `json:"leftupper"`
    }{
        Quad:               inner.quad,
        ChildLeftLower:     inner.childLeftLower,
        ChildRightLower:    inner.childRightLower,
        ChildRightUpper:    inner.childRightUpper,
        ChildLeftUpper:     inner.childLeftUpper,
    })
}

func (equal *equalNode) MarshalJSON() ([]byte, error) {
    return json.Marshal(&struct {
        Quad                Quad            `json:"quad"`
        Entry               quadTreeEntry   `json:"entry"`
        Next                quadTreeNode    `json:"next"`
    }{
        Quad:               equal.quad,
        Entry:              equal.entry,
        Next:               equal.next,
    })
}
