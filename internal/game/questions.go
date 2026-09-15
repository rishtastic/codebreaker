package game

import (
	"fmt"
	"strings"
)

type QuestionKind string

const (
	CountSame     QuestionKind = "count_same"
	CountColor    QuestionKind = "count_color"
	CountParity   QuestionKind = "count_parity"
	Consecutive   QuestionKind = "consecutive"
	SameColor     QuestionKind = "same_color"
	SumRange      QuestionKind = "sum_range"
	Difference    QuestionKind = "difference"
	SumAll        QuestionKind = "sum_all"
	SumColor      QuestionKind = "sum_color"
	Locate        QuestionKind = "locate"
	MiddleGreater QuestionKind = "middle_greater"
)

type Question struct {
	ID      string       `json:"id"`
	Prompt  string       `json:"prompt"`
	Kind    QuestionKind `json:"-"`
	Choices []int        `json:"choices,omitempty"`
	Color   Color        `json:"-"`
	Parity  int          `json:"-"`
	Start   int          `json:"-"`
	End     int          `json:"-"`
}

func AllQuestions() []Question {
	return []Question{
		{ID: "same-number", Prompt: "How many pairs of tiles have the same number?", Kind: CountSame},
		{ID: "black-count", Prompt: "How many black tiles do you have?", Kind: CountColor, Color: Black},
		{ID: "white-count", Prompt: "How many white tiles do you have?", Kind: CountColor, Color: White},
		{ID: "even-count", Prompt: "How many even-numbered tiles do you have?", Kind: CountParity, Parity: 0},
		{ID: "odd-count", Prompt: "How many odd-numbered tiles do you have?", Kind: CountParity, Parity: 1},
		{ID: "consecutive", Prompt: "Which neighboring tiles have consecutive numbers?", Kind: Consecutive},
		{ID: "same-color", Prompt: "Which neighboring tiles have the same color?", Kind: SameColor},
		{ID: "left-sum", Prompt: "What is the sum of your three leftmost tiles?", Kind: SumRange, Start: 0, End: 3},
		{ID: "middle-sum", Prompt: "What is the sum of tiles B, C and D?", Kind: SumRange, Start: 1, End: 4},
		{ID: "right-sum", Prompt: "What is the sum of your three rightmost tiles?", Kind: SumRange, Start: 2, End: 5},
		{ID: "difference", Prompt: "What is the difference between your highest and lowest numbers?", Kind: Difference},
		{ID: "total-sum", Prompt: "What is the sum of all your tiles?", Kind: SumAll},
		{ID: "black-sum", Prompt: "What is the sum of your black tiles?", Kind: SumColor, Color: Black},
		{ID: "white-sum", Prompt: "What is the sum of your white tiles?", Kind: SumColor, Color: White},
		{ID: "locate-0", Prompt: "Where are your number 0 tiles?", Kind: Locate, Choices: []int{0}},
		{ID: "locate-5", Prompt: "Where are your number 5 tiles?", Kind: Locate, Choices: []int{5}},
		{ID: "locate-1-2", Prompt: "Where are your number {number} tiles?", Kind: Locate, Choices: []int{1, 2}},
		{ID: "locate-3-4", Prompt: "Where are your number {number} tiles?", Kind: Locate, Choices: []int{3, 4}},
		{ID: "middle-over-4", Prompt: "Is your middle tile greater than 4?", Kind: MiddleGreater},
		{ID: "locate-6-7", Prompt: "Where are your number {number} tiles?", Kind: Locate, Choices: []int{6, 7}},
		{ID: "locate-8-9", Prompt: "Where are your number {number} tiles?", Kind: Locate, Choices: []int{8, 9}},
	}
}

func (q Question) Accepts(choice *int) bool {
	if len(q.Choices) == 0 {
		return choice == nil
	}
	if len(q.Choices) == 1 && choice == nil {
		return true
	}
	if choice == nil {
		return false
	}
	for _, option := range q.Choices {
		if option == *choice {
			return true
		}
	}
	return false
}

func (q Question) Answer(tiles []Tile, choice *int) string {
	switch q.Kind {
	case CountSame:
		counts := map[int]int{}
		for _, tile := range tiles {
			counts[tile.Number]++
		}
		total := 0
		for _, count := range counts {
			if count == 2 {
				total++
			}
		}
		if total == 1 {
			return "There is 1 pair."
		}
		return fmt.Sprintf("There are %d pairs.", total)
	case CountColor:
		total := 0
		for _, tile := range tiles {
			if tile.Color == q.Color {
				total++
			}
		}
		return countAnswer(total)
	case CountParity:
		total := 0
		for _, tile := range tiles {
			if tile.Number%2 == q.Parity {
				total++
			}
		}
		return countAnswer(total)
	case Consecutive:
		return groupAnswer(groups(tiles, func(a, b Tile) bool { return b.Number == a.Number+1 }))
	case SameColor:
		return groupAnswer(groups(tiles, func(a, b Tile) bool { return a.Color == b.Color }))
	case SumRange:
		return fmt.Sprintf("The sum is %d.", sum(tiles[q.Start:q.End], ""))
	case Difference:
		return fmt.Sprintf("The difference is %d.", tiles[len(tiles)-1].Number-tiles[0].Number)
	case SumAll:
		return fmt.Sprintf("The sum is %d.", sum(tiles, ""))
	case SumColor:
		return fmt.Sprintf("The sum is %d.", sum(tiles, q.Color))
	case Locate:
		number := q.Choices[0]
		if choice != nil {
			number = *choice
		}
		positions := []string{}
		for i, tile := range tiles {
			if tile.Number == number {
				positions = append(positions, position(i))
			}
		}
		if len(positions) == 0 {
			return fmt.Sprintf("There are no number %d tiles.", number)
		}
		return fmt.Sprintf("Number %d is at %s.", number, joinWords(positions))
	case MiddleGreater:
		if tiles[2].Number > 4 {
			return "Yes."
		}
		return "No."
	default:
		return ""
	}
}

func countAnswer(count int) string {
	if count == 1 {
		return "There is 1 tile."
	}
	return fmt.Sprintf("There are %d tiles.", count)
}

func sum(tiles []Tile, color Color) int {
	total := 0
	for _, tile := range tiles {
		if color == "" || tile.Color == color {
			total += tile.Number
		}
	}
	return total
}

func groups(tiles []Tile, connected func(Tile, Tile) bool) [][]string {
	var result [][]string
	for start := 0; start < len(tiles)-1; {
		if !connected(tiles[start], tiles[start+1]) {
			start++
			continue
		}
		end := start + 1
		for end < len(tiles)-1 && connected(tiles[end], tiles[end+1]) {
			end++
		}
		group := make([]string, 0, end-start+1)
		for i := start; i <= end; i++ {
			group = append(group, position(i))
		}
		result = append(result, group)
		start = end + 1
	}
	return result
}

func groupAnswer(groups [][]string) string {
	if len(groups) == 0 {
		return "There are none."
	}
	parts := make([]string, len(groups))
	for i, group := range groups {
		parts[i] = joinWords(group)
	}
	return strings.Join(parts, "; ") + "."
}

func position(index int) string { return string(rune('A' + index)) }

func joinWords(values []string) string {
	if len(values) == 1 {
		return values[0]
	}
	if len(values) == 2 {
		return values[0] + " and " + values[1]
	}
	return strings.Join(values[:len(values)-1], ", ") + " and " + values[len(values)-1]
}
