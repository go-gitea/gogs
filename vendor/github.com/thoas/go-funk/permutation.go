package funk

import "errors"

// NextPermutation Implement next permutation,
// which rearranges numbers into the lexicographically next greater permutation of numbers.
func NextPermutation(nums []int) error {
	n := len(nums)
	if n == 0 {
		return errors.New("nums is empty")
	}

	i := n - 2

	for i >= 0 && nums[i] >= nums[i+1] {
		i--
	}

	if i >= 0 {
		j := n - 1
		for j >= 0 && nums[i] >= nums[j] {
			j--
		}
		nums[i], nums[j] = nums[j], nums[i]
	}

	ReverseInt(nums[i+1:])
	return nil
}
