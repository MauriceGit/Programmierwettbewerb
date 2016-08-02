package main

import (
    . "Programmierwettbewerb-Server/vector"
    . "Programmierwettbewerb-Server/shared"
    . "Programmierwettbewerb-Server/data"
    
    "os"
    "io/ioutil"
    "encoding/json"
    "fmt"
)

func main() {
    SetLoggingDebug(true)
    SetLoggingVerbose(false)
    
    args := os.Args[1:]
    
    foods := make(map[string]Food)
    
    filename := args[0]
    
    file, e := ioutil.ReadFile(filename)
    if e != nil {
        Logf(LtDebug, "Could not read the file: %v\n", filename)
        return
    }
    json.Unmarshal(file, &foods)
    
    quad := NewQuad(Vec2{ 0, 0 }, 1000)
    allocator := NewAllocator(10000, 100, 5000, 10000)
    quadTree := NewQuadTree(quad, &allocator)
    
    for _, food := range foods {
        quadTree.Insert(food.Position, food)
    }
    
    quadTreeInfo := quadTree.GetInfo()
    
    Logf(LtDebug, "Tree: \n")
    Logf(LtDebug, "EmptyNodeCount: %v\n", quadTreeInfo.EmptyNodeCount)
    Logf(LtDebug, "EqualNodeCount: %v\n", quadTreeInfo.EqualNodeCount)
    Logf(LtDebug, "InnerNodeCount: %v\n", quadTreeInfo.InnerNodeCount)
    Logf(LtDebug, "LeafNodeCount: %v\n", quadTreeInfo.LeafNodeCount)
    
    Logf(LtDebug, "Allocator: \n")
    allocator.Report()
    
    fmt.Printf("Success!\n")
}
    
