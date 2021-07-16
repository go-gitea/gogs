package funk

type JoinIntFnc func(lx, rx []int) []int

// JoinInt combines two int collections using the given join method.
func JoinInt(larr, rarr []int, fnc JoinIntFnc) []int {
	return fnc(larr, rarr)
}

// InnerJoinInt finds and returns matching data from two int collections.
func InnerJoinInt(lx, rx []int) []int {
	result := make([]int, 0, len(lx)+len(rx))
	rhash := hashSliceInt(rx)
	lhash := make(map[int]struct{}, len(lx))

	for _, v := range lx {
		_, ok := rhash[v]
		_, alreadyExists := lhash[v]
		if ok && !alreadyExists {
			lhash[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// OuterJoinInt finds and returns dissimilar data from two int collections.
func OuterJoinInt(lx, rx []int) []int {
	ljoin := LeftJoinInt(lx, rx)
	rjoin := RightJoinInt(lx, rx)

	result := make([]int, len(ljoin)+len(rjoin))
	for i, v := range ljoin {
		result[i] = v
	}
	for i, v := range rjoin {
		result[len(ljoin)+i] = v
	}
	return result
}

// LeftJoinInt finds and returns dissimilar data from the first int collection (left).
func LeftJoinInt(lx, rx []int) []int {
	result := make([]int, 0, len(lx))
	rhash := hashSliceInt(rx)

	for _, v := range lx {
		_, ok := rhash[v]
		if !ok {
			result = append(result, v)
		}
	}
	return result
}

// LeftJoinInt finds and returns dissimilar data from the second int collection (right).
func RightJoinInt(lx, rx []int) []int { return LeftJoinInt(rx, lx) }

func hashSliceInt(arr []int) map[int]struct{} {
	hash := make(map[int]struct{}, len(arr))
	for _, i := range arr {
		hash[i] = struct{}{}
	}
	return hash
}

type JoinInt32Fnc func(lx, rx []int32) []int32

// JoinInt32 combines two int32 collections using the given join method.
func JoinInt32(larr, rarr []int32, fnc JoinInt32Fnc) []int32 {
	return fnc(larr, rarr)
}

// InnerJoinInt32 finds and returns matching data from two int32 collections.
func InnerJoinInt32(lx, rx []int32) []int32 {
	result := make([]int32, 0, len(lx)+len(rx))
	rhash := hashSliceInt32(rx)
	lhash := make(map[int32]struct{}, len(lx))

	for _, v := range lx {
		_, ok := rhash[v]
		_, alreadyExists := lhash[v]
		if ok && !alreadyExists {
			lhash[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// OuterJoinInt32 finds and returns dissimilar data from two int32 collections.
func OuterJoinInt32(lx, rx []int32) []int32 {
	ljoin := LeftJoinInt32(lx, rx)
	rjoin := RightJoinInt32(lx, rx)

	result := make([]int32, len(ljoin)+len(rjoin))
	for i, v := range ljoin {
		result[i] = v
	}
	for i, v := range rjoin {
		result[len(ljoin)+i] = v
	}
	return result
}

// LeftJoinInt32 finds and returns dissimilar data from the first int32 collection (left).
func LeftJoinInt32(lx, rx []int32) []int32 {
	result := make([]int32, 0, len(lx))
	rhash := hashSliceInt32(rx)

	for _, v := range lx {
		_, ok := rhash[v]
		if !ok {
			result = append(result, v)
		}
	}
	return result
}

// LeftJoinInt32 finds and returns dissimilar data from the second int32 collection (right).
func RightJoinInt32(lx, rx []int32) []int32 { return LeftJoinInt32(rx, lx) }

func hashSliceInt32(arr []int32) map[int32]struct{} {
	hash := make(map[int32]struct{}, len(arr))
	for _, i := range arr {
		hash[i] = struct{}{}
	}
	return hash
}

type JoinInt64Fnc func(lx, rx []int64) []int64

// JoinInt64 combines two int64 collections using the given join method.
func JoinInt64(larr, rarr []int64, fnc JoinInt64Fnc) []int64 {
	return fnc(larr, rarr)
}

// InnerJoinInt64 finds and returns matching data from two int64 collections.
func InnerJoinInt64(lx, rx []int64) []int64 {
	result := make([]int64, 0, len(lx)+len(rx))
	rhash := hashSliceInt64(rx)
	lhash := make(map[int64]struct{}, len(lx))

	for _, v := range lx {
		_, ok := rhash[v]
		_, alreadyExists := lhash[v]
		if ok && !alreadyExists {
			lhash[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// OuterJoinInt64 finds and returns dissimilar data from two int64 collections.
func OuterJoinInt64(lx, rx []int64) []int64 {
	ljoin := LeftJoinInt64(lx, rx)
	rjoin := RightJoinInt64(lx, rx)

	result := make([]int64, len(ljoin)+len(rjoin))
	for i, v := range ljoin {
		result[i] = v
	}
	for i, v := range rjoin {
		result[len(ljoin)+i] = v
	}
	return result
}

// LeftJoinInt64 finds and returns dissimilar data from the first int64 collection (left).
func LeftJoinInt64(lx, rx []int64) []int64 {
	result := make([]int64, 0, len(lx))
	rhash := hashSliceInt64(rx)

	for _, v := range lx {
		_, ok := rhash[v]
		if !ok {
			result = append(result, v)
		}
	}
	return result
}

// LeftJoinInt64 finds and returns dissimilar data from the second int64 collection (right).
func RightJoinInt64(lx, rx []int64) []int64 { return LeftJoinInt64(rx, lx) }

func hashSliceInt64(arr []int64) map[int64]struct{} {
	hash := make(map[int64]struct{}, len(arr))
	for _, i := range arr {
		hash[i] = struct{}{}
	}
	return hash
}

type JoinStringFnc func(lx, rx []string) []string

// JoinString combines two string collections using the given join method.
func JoinString(larr, rarr []string, fnc JoinStringFnc) []string {
	return fnc(larr, rarr)
}

// InnerJoinString finds and returns matching data from two string collections.
func InnerJoinString(lx, rx []string) []string {
	result := make([]string, 0, len(lx)+len(rx))
	rhash := hashSliceString(rx)
	lhash := make(map[string]struct{}, len(lx))

	for _, v := range lx {
		_, ok := rhash[v]
		_, alreadyExists := lhash[v]
		if ok && !alreadyExists {
			lhash[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// OuterJoinString finds and returns dissimilar data from two string collections.
func OuterJoinString(lx, rx []string) []string {
	ljoin := LeftJoinString(lx, rx)
	rjoin := RightJoinString(lx, rx)

	result := make([]string, len(ljoin)+len(rjoin))
	for i, v := range ljoin {
		result[i] = v
	}
	for i, v := range rjoin {
		result[len(ljoin)+i] = v
	}
	return result
}

// LeftJoinString finds and returns dissimilar data from the first string collection (left).
func LeftJoinString(lx, rx []string) []string {
	result := make([]string, 0, len(lx))
	rhash := hashSliceString(rx)

	for _, v := range lx {
		_, ok := rhash[v]
		if !ok {
			result = append(result, v)
		}
	}
	return result
}

// LeftJoinString finds and returns dissimilar data from the second string collection (right).
func RightJoinString(lx, rx []string) []string { return LeftJoinString(rx, lx) }

func hashSliceString(arr []string) map[string]struct{} {
	hash := make(map[string]struct{}, len(arr))
	for _, i := range arr {
		hash[i] = struct{}{}
	}
	return hash
}

type JoinFloat32Fnc func(lx, rx []float32) []float32

// JoinFloat32 combines two float32 collections using the given join method.
func JoinFloat32(larr, rarr []float32, fnc JoinFloat32Fnc) []float32 {
	return fnc(larr, rarr)
}

// InnerJoinFloat32 finds and returns matching data from two float32 collections.
func InnerJoinFloat32(lx, rx []float32) []float32 {
	result := make([]float32, 0, len(lx)+len(rx))
	rhash := hashSliceFloat32(rx)
	lhash := make(map[float32]struct{}, len(lx))

	for _, v := range lx {
		_, ok := rhash[v]
		_, alreadyExists := lhash[v]
		if ok && !alreadyExists {
			lhash[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// OuterJoinFloat32 finds and returns dissimilar data from two float32 collections.
func OuterJoinFloat32(lx, rx []float32) []float32 {
	ljoin := LeftJoinFloat32(lx, rx)
	rjoin := RightJoinFloat32(lx, rx)

	result := make([]float32, len(ljoin)+len(rjoin))
	for i, v := range ljoin {
		result[i] = v
	}
	for i, v := range rjoin {
		result[len(ljoin)+i] = v
	}
	return result
}

// LeftJoinFloat32 finds and returns dissimilar data from the first float32 collection (left).
func LeftJoinFloat32(lx, rx []float32) []float32 {
	result := make([]float32, 0, len(lx))
	rhash := hashSliceFloat32(rx)

	for _, v := range lx {
		_, ok := rhash[v]
		if !ok {
			result = append(result, v)
		}
	}
	return result
}

// LeftJoinFloat32 finds and returns dissimilar data from the second float32 collection (right).
func RightJoinFloat32(lx, rx []float32) []float32 { return LeftJoinFloat32(rx, lx) }

func hashSliceFloat32(arr []float32) map[float32]struct{} {
	hash := make(map[float32]struct{}, len(arr))
	for _, i := range arr {
		hash[i] = struct{}{}
	}
	return hash
}

type JoinFloat64Fnc func(lx, rx []float64) []float64

// JoinFloat64 combines two float64 collections using the given join method.
func JoinFloat64(larr, rarr []float64, fnc JoinFloat64Fnc) []float64 {
	return fnc(larr, rarr)
}

// InnerJoinFloat64 finds and returns matching data from two float64 collections.
func InnerJoinFloat64(lx, rx []float64) []float64 {
	result := make([]float64, 0, len(lx)+len(rx))
	rhash := hashSliceFloat64(rx)
	lhash := make(map[float64]struct{}, len(lx))

	for _, v := range lx {
		_, ok := rhash[v]
		_, alreadyExists := lhash[v]
		if ok && !alreadyExists {
			lhash[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// OuterJoinFloat64 finds and returns dissimilar data from two float64 collections.
func OuterJoinFloat64(lx, rx []float64) []float64 {
	ljoin := LeftJoinFloat64(lx, rx)
	rjoin := RightJoinFloat64(lx, rx)

	result := make([]float64, len(ljoin)+len(rjoin))
	for i, v := range ljoin {
		result[i] = v
	}
	for i, v := range rjoin {
		result[len(ljoin)+i] = v
	}
	return result
}

// LeftJoinFloat64 finds and returns dissimilar data from the first float64 collection (left).
func LeftJoinFloat64(lx, rx []float64) []float64 {
	result := make([]float64, 0, len(lx))
	rhash := hashSliceFloat64(rx)

	for _, v := range lx {
		_, ok := rhash[v]
		if !ok {
			result = append(result, v)
		}
	}
	return result
}

// LeftJoinFloat64 finds and returns dissimilar data from the second float64 collection (right).
func RightJoinFloat64(lx, rx []float64) []float64 { return LeftJoinFloat64(rx, lx) }

func hashSliceFloat64(arr []float64) map[float64]struct{} {
	hash := make(map[float64]struct{}, len(arr))
	for _, i := range arr {
		hash[i] = struct{}{}
	}
	return hash
}
