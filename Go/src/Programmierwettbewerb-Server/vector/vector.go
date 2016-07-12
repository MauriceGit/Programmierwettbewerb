package vector

import (
    "math"
    "math/rand"
)

type Vec2 struct {
    X float32
    Y float32
}

type Vec3 struct {
    X float32
    Y float32
    Z float32
}

func Add(lhs Vec2, rhs Vec2) Vec2 {
    return Vec2{ lhs.X + rhs.X, lhs.Y + rhs.Y }
}

func Sub(lhs Vec2, rhs Vec2) Vec2 {
    return Vec2{ lhs.X - rhs.X, lhs.Y - rhs.Y }
}

func Muls(lhs Vec2, rhs float32) Vec2 {
    return Vec2{ lhs.X * rhs, lhs.Y * rhs }
}

func Mulv(lhs Vec2, rhs Vec2) Vec2 {
    return Vec2{ lhs.X * rhs.X, lhs.Y * rhs.Y }
}

func Length(v Vec2) float32 {
    return float32(math.Sqrt(float64(v.X * v.X + v.Y * v.Y)))
}

func LengthSquared(v Vec2) float32 {
    return float32(v.X * v.X + v.Y * v.Y)
}

func Dist(pos1 Vec2, pos2 Vec2) float32 {
    return Length(Sub(pos1, pos2))
}

func DistFast(pos1 Vec2, pos2 Vec2) float32 {
    return LengthSquared(Sub(pos1, pos2))
}

func Normalize(v Vec2) Vec2 {
    var length = Length(v)
    return Vec2{ v.X / length, v.Y / length }
}

func NormalizeOrZero(v Vec2) Vec2 {
    var length = Length(v)
    if length > 0 {
        return Vec2{ v.X / length, v.Y / length }
    }
    return Vec2{ 0, 0 }
}

func NullVec2() Vec2 {
    return Vec2{ 0.0, 0.0 }
}

func RandomVec2() Vec2 {
    return Vec2{ rand.Float32(), rand.Float32() }
}

func CopyVec2(v Vec2) Vec2 {
    return Vec2{ v.X, v.Y }
}

func RandomVec2Unit() Vec2 {
    return Vec2{ 2*rand.Float32() - 1, 2*rand.Float32() - 1 }
}

func RandomVec2in(min, max Vec2) Vec2 {
    return Add(min, Mulv(RandomVec2(), Sub(max, min)))
}
