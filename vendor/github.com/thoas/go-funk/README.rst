go-funk
=======

.. image:: https://secure.travis-ci.org/thoas/go-funk.svg?branch=master
    :alt: Build Status
    :target: http://travis-ci.org/thoas/go-funk

.. image:: https://godoc.org/github.com/thoas/go-funk?status.svg
    :alt: GoDoc
    :target: https://pkg.go.dev/github.com/thoas/go-funk

.. image:: https://goreportcard.com/badge/github.com/thoas/go-funk
    :alt: Go report
    :target: https://goreportcard.com/report/github.com/thoas/go-funk

``go-funk`` is a modern Go library based on reflect_.

Generic helpers rely on reflect_, be careful this code runs exclusively on runtime so you must have a good test suite.

These helpers have started as an experiment to learn reflect_. It may look like lodash_ in some aspects but
it will have its own roadmap. lodash_ is an awesome library with a lot of work behind it, all features included in
``go-funk`` come from internal use cases.

You can also find typesafe implementation in the godoc_.

Why this name?
--------------

Long story, short answer because ``func`` is a reserved word in Go, I wanted something similar.

Initially this project was named ``fn`` I don't need to explain why that was a bad idea for french speakers :)

Let's ``funk``!

.. image:: https://media.giphy.com/media/3oEjHQKtDXpeGN9rW0/giphy.gif

<3

Installation
------------

.. code-block:: bash

    go get github.com/thoas/go-funk

Usage
-----

.. code-block:: go

    import "github.com/thoas/go-funk"

These examples will be based on the following data model:

.. code-block:: go

    type Foo struct {
        ID        int
        FirstName string `tag_name:"tag 1"`
        LastName  string `tag_name:"tag 2"`
        Age       int    `tag_name:"tag 3"`
    }

    func (f Foo) TableName() string {
        return "foo"
    }

With fixtures:

.. code-block:: go

    f := &Foo{
        ID:        1,
        FirstName: "Foo",
        LastName:  "Bar",
        Age:       30,
    }

You can import ``go-funk`` using a basic statement:

.. code-block:: go

    import "github.com/thoas/go-funk"

funk.Contains
.............

Returns true if an element is present in a iteratee (slice, map, string).

One frustrating thing in Go is to implement ``contains`` methods for each type, for example:

.. code-block:: go

    func ContainsInt(s []int, e int) bool {
        for _, a := range s {
            if a == e {
                return true
            }
        }
        return false
    }

this can be replaced by ``funk.Contains``:

.. code-block:: go

    // slice of string
    funk.Contains([]string{"foo", "bar"}, "bar") // true

    // slice of Foo ptr
    funk.Contains([]*Foo{f}, f) // true
    funk.Contains([]*Foo{f}, func (foo *Foo) bool {
        return foo.ID == f.ID
    }) // true
    funk.Contains([]*Foo{f}, nil) // false

    b := &Foo{
        ID:        2,
        FirstName: "Florent",
        LastName:  "Messa",
        Age:       28,
    }

    funk.Contains([]*Foo{f}, b) // false

    // string
    funk.Contains("florent", "rent") // true
    funk.Contains("florent", "foo") // false

    // even map
    funk.Contains(map[int]string{1: "Florent"}, 1) // true
    funk.Contains(map[int]string{1: "Florent"}, func(key int, name string) bool {
        return key == 1 // or `name == "Florent"` for the value type
    }) // true

see also, typesafe implementations: ContainsInt_, ContainsInt64_, ContainsFloat32_, ContainsFloat64_, ContainsString_

.. _ContainsFloat32: https://godoc.org/github.com/thoas/go-funk#ContainsFloat32
.. _ContainsFloat64: https://godoc.org/github.com/thoas/go-funk#ContainsFloat64
.. _ContainsInt: https://godoc.org/github.com/thoas/go-funk#ContainsInt
.. _ContainsInt64: https://godoc.org/github.com/thoas/go-funk#ContainsInt64
.. _ContainsString: https://godoc.org/github.com/thoas/go-funk#ContainsString

funk.Intersect
..............

Returns the intersection between two collections.

.. code-block:: go

    funk.Intersect([]int{1, 2, 3, 4}, []int{2, 4, 6})  // []int{2, 4}
    funk.Intersect([]string{"foo", "bar", "hello", "bar"}, []string{"foo", "bar"})  // []string{"foo", "bar"}

see also, typesafe implementations: IntersectString

.. IntersectString: https://godoc.org/github.com/thoas/go-funk#IntersectString


funk.Difference
..............

Returns the difference between two collections.

.. code-block:: go

    funk.Difference([]int{1, 2, 3, 4}, []int{2, 4, 6})  // []int{1, 3}, []int{6}
    funk.Difference([]string{"foo", "bar", "hello", "bar"}, []string{"foo", "bar"})  // []string{"hello"}, []string{}

see also, typesafe implementations: DifferenceString

.. DifferenceString: https://godoc.org/github.com/thoas/go-funk#DifferenceString


funk.IndexOf
............

Gets the index at which the first occurrence of a value is found in an array or return -1
if the value cannot be found.

.. code-block:: go

    // slice of string
    funk.IndexOf([]string{"foo", "bar"}, "bar") // 1
    funk.IndexOf([]string{"foo", "bar"}, func(value string) bool {
        return value == "bar"
    }) // 1
    funk.IndexOf([]string{"foo", "bar"}, "gilles") // -1

see also, typesafe implementations: IndexOfInt_, IndexOfInt64_, IndexOfFloat32_, IndexOfFloat64_, IndexOfString_

.. _IndexOfFloat32: https://godoc.org/github.com/thoas/go-funk#IndexOfFloat32
.. _IndexOfFloat64: https://godoc.org/github.com/thoas/go-funk#IndexOfFloat64
.. _IndexOfInt: https://godoc.org/github.com/thoas/go-funk#IndexOfInt
.. _IndexOfInt64: https://godoc.org/github.com/thoas/go-funk#IndexOfInt64
.. _IndexOfString: https://godoc.org/github.com/thoas/go-funk#IndexOfString

funk.LastIndexOf
................

Gets the index at which the last occurrence of a value is found in an array or return -1
if the value cannot be found.

.. code-block:: go

    // slice of string
    funk.LastIndexOf([]string{"foo", "bar", "bar"}, "bar") // 2
    funk.LastIndexOf([]string{"foo", "bar"}, func(value string) bool {
        return value == "bar"
    }) // 2
    funk.LastIndexOf([]string{"foo", "bar"}, "gilles") // -1

see also, typesafe implementations: LastIndexOfInt_, LastIndexOfInt64_, LastIndexOfFloat32_, LastIndexOfFloat64_, LastIndexOfString_

.. _LastIndexOfFloat32: https://godoc.org/github.com/thoas/go-funk#LastIndexOfFloat32
.. _LastIndexOfFloat64: https://godoc.org/github.com/thoas/go-funk#LastIndexOfFloat64
.. _LastIndexOfInt: https://godoc.org/github.com/thoas/go-funk#LastIndexOfInt
.. _LastIndexOfInt64: https://godoc.org/github.com/thoas/go-funk#LastIndexOfInt64
.. _LastIndexOfString: https://godoc.org/github.com/thoas/go-funk#LastIndexOfString

funk.ToMap
..........

Transforms a slice of structs to a map based on a ``pivot`` field.

.. code-block:: go

    f := &Foo{
        ID:        1,
        FirstName: "Gilles",
        LastName:  "Fabio",
        Age:       70,
    }

    b := &Foo{
        ID:        2,
        FirstName: "Florent",
        LastName:  "Messa",
        Age:       80,
    }

    results := []*Foo{f, b}

    mapping := funk.ToMap(results, "ID") // map[int]*Foo{1: f, 2: b}

funk.Filter
...........

Filters a slice based on a predicate.

.. code-block:: go

    r := funk.Filter([]int{1, 2, 3, 4}, func(x int) bool {
        return x%2 == 0
    }) // []int{2, 4}

see also, typesafe implementations: FilterInt_, FilterInt64_, FilterFloat32_, FilterFloat64_, FilterString_

.. _FilterFloat32: https://godoc.org/github.com/thoas/go-funk#FilterFloat32
.. _FilterFloat64: https://godoc.org/github.com/thoas/go-funk#FilterFloat64
.. _FilterInt: https://godoc.org/github.com/thoas/go-funk#FilterInt
.. _FilterInt64: https://godoc.org/github.com/thoas/go-funk#FilterInt64
.. _FilterString: https://godoc.org/github.com/thoas/go-funk#FilterString

funk.Reduce
...........

Reduces an iteratee based on an accumulator function or operation rune for numbers.

.. code-block:: go

    // Using operation runes. '+' and '*' only supported.
    r := funk.Reduce([]int{1, 2, 3, 4}, '+', float64(0)) // 10
    r := funk.Reduce([]int{1, 2, 3, 4}, '*', 1) // 24

    // Using accumulator function
    r := funk.Reduce([]int{1, 2, 3, 4}, func(acc float64, num int) float64 {
        return acc + float64(num)
    }, float64(0)) // 10

    r := funk.Reduce([]int{1, 2, 3, 4}, func(acc string, num int) string {
        return acc + fmt.Sprint(num)
    }, "") // "1234"

funk.Find
.........

Finds an element in a slice based on a predicate.

.. code-block:: go

    r := funk.Find([]int{1, 2, 3, 4}, func(x int) bool {
        return x%2 == 0
    }) // 2

see also, typesafe implementations: FindInt_, FindInt64_, FindFloat32_, FindFloat64_, FindString_

.. _FindFloat32: https://godoc.org/github.com/thoas/go-funk#FindFloat32
.. _FindFloat64: https://godoc.org/github.com/thoas/go-funk#FindFloat64
.. _FindInt: https://godoc.org/github.com/thoas/go-funk#FindInt
.. _FindInt64: https://godoc.org/github.com/thoas/go-funk#FindInt64
.. _FindString: https://godoc.org/github.com/thoas/go-funk#FindString

funk.Map
........

Manipulates an iteratee (map, slice) and transforms it to another type:

* map -> slice
* map -> map
* slice -> map
* slice -> slice

.. code-block:: go

    r := funk.Map([]int{1, 2, 3, 4}, func(x int) int {
        return x * 2
    }) // []int{2, 4, 6, 8}

    r := funk.Map([]int{1, 2, 3, 4}, func(x int) string {
        return "Hello"
    }) // []string{"Hello", "Hello", "Hello", "Hello"}

    r = funk.Map([]int{1, 2, 3, 4}, func(x int) (int, int) {
        return x, x
    }) // map[int]int{1: 1, 2: 2, 3: 3, 4: 4}

    mapping := map[int]string{
        1: "Florent",
        2: "Gilles",
    }

    r = funk.Map(mapping, func(k int, v string) int {
        return k
    }) // []int{1, 2}

    r = funk.Map(mapping, func(k int, v string) (string, string) {
        return fmt.Sprintf("%d", k), v
    }) // map[string]string{"1": "Florent", "2": "Gilles"}

funk.FlatMap
............

Manipulates an iteratee (map, slice) and transforms it to to a flattened collection of another type:

* map -> slice
* slice -> slice

.. code-block:: go

    r := funk.FlatMap([][]int{{1, 2}, {3, 4}}, func(x []int) []int {
        return append(x, 0)
    }) // []int{1, 2, 0, 3, 4, 0}

    mapping := map[string][]int{
        "Florent": {1, 2},
        "Gilles": {3, 4},
    }

    r = funk.FlatMap(mapping, func(k string, v []int) []int {
        return v
    }) // []int{1, 2, 3, 4}

funk.Get
........

Retrieves the value at path of struct(s) or map(s).

.. code-block:: go

    var bar *Bar = &Bar{
        Name: "Test",
        Bars: []*Bar{
            &Bar{
                Name: "Level1-1",
                Bar: &Bar{
                    Name: "Level2-1",
                },
            },
            &Bar{
                Name: "Level1-2",
                Bar: &Bar{
                    Name: "Level2-2",
                },
            },
        },
    }

    var foo *Foo = &Foo{
        ID:        1,
        FirstName: "Dark",
        LastName:  "Vador",
        Age:       30,
        Bar:       bar,
        Bars: []*Bar{
            bar,
            bar,
        },
    }

    funk.Get([]*Foo{foo}, "Bar.Bars.Bar.Name") // []string{"Level2-1", "Level2-2"}
    funk.Get(foo, "Bar.Bars.Bar.Name") // []string{"Level2-1", "Level2-2"}
    funk.Get(foo, "Bar.Name") // Test

``funk.Get`` also support ``map`` values:

.. code-block:: go

    bar := map[string]interface{}{
        "Name": "Test",
    }

    foo1 := map[string]interface{}{
        "ID":        1,
        "FirstName": "Dark",
        "LastName":  "Vador",
        "Age":       30,
        "Bar":       bar,
    }

    foo2 := &map[string]interface{}{
        "ID":        1,
        "FirstName": "Dark",
        "LastName":  "Vador",
        "Age":       30,
    } // foo2.Bar is nil

    funk.Get(bar, "Name") // "Test"
    funk.Get([]map[string]interface{}{foo1, foo2}, "Bar.Name") // []string{"Test"}
    funk.Get(foo2, "Bar.Name") // nil


``funk.Get`` also handles ``nil`` values:

.. code-block:: go

    bar := &Bar{
        Name: "Test",
    }

    foo1 := &Foo{
        ID:        1,
        FirstName: "Dark",
        LastName:  "Vador",
        Age:       30,
        Bar:       bar,
    }

    foo2 := &Foo{
        ID:        1,
        FirstName: "Dark",
        LastName:  "Vador",
        Age:       30,
    } // foo2.Bar is nil

    funk.Get([]*Foo{foo1, foo2}, "Bar.Name") // []string{"Test"}
    funk.Get(foo2, "Bar.Name") // nil



funk.GetOrElse
..............

Retrieves the value of the pointer or default.

.. code-block:: go

    str := "hello world"
    GetOrElse(&str, "foobar")   // string{"hello world"}
    GetOrElse(str, "foobar")    // string{"hello world"}
    GetOrElse(nil, "foobar")    // string{"foobar"}

funk.Set
........
Set value at a path of a struct

.. code-block:: go

    var bar Bar = Bar{
        Name: "level-0", 
        Bar: &Bar{
            Name: "level-1",
            Bars: []*Bar{
                {Name: "level2-1"},
                {Name: "level2-2"},
            },
        },
    }

    _ = Set(&bar, "level-0-new", "Name")
    fmt.Println(bar.Name) // "level-0-new"

    MustSet(&bar, "level-1-new", "Bar.Name")
    fmt.Println(bar.Bar.Name) // "level-1-new"

    Set(&bar, "level-2-new", "Bar.Bars.Name")
    fmt.Println(bar.Bar.Bars[0].Name) // "level-2-new"
    fmt.Println(bar.Bar.Bars[1].Name) // "level-2-new"

funk.MustSet
............
Short hand for funk.Set if struct does not contain interface{} field type to discard errors.

funk.Prune
..........
Copy a struct with only selected fields. Slice is handled by pruning all elements.

.. code-block:: go

    bar := &Bar{
        Name: "Test",
    }

    foo1 := &Foo{
        ID:        1,
        FirstName: "Dark",
        LastName:  "Vador",
        Bar:       bar,
    }

    pruned, _ := Prune(foo1, []string{"FirstName", "Bar.Name"})
    // *Foo{
    //    ID:        0,
    //    FirstName: "Dark",
    //    LastName:  "",
    //    Bar:       &Bar{Name: "Test},
    // }

funk.PruneByTag
..........
Same functionality as funk.Prune, but uses struct tags instead of struct field names.

funk.Keys
.........

Creates an array of the own enumerable map keys or struct field names.

.. code-block:: go

    funk.Keys(map[string]int{"one": 1, "two": 2}) // []string{"one", "two"} (iteration order is not guaranteed)

    foo := &Foo{
        ID:        1,
        FirstName: "Dark",
        LastName:  "Vador",
        Age:       30,
    }

    funk.Keys(foo) // []string{"ID", "FirstName", "LastName", "Age"} (iteration order is not guaranteed)

funk.Values
...........

Creates an array of the own enumerable map values or struct field values.

.. code-block:: go

    funk.Values(map[string]int{"one": 1, "two": 2}) // []string{1, 2} (iteration order is not guaranteed)

    foo := &Foo{
        ID:        1,
        FirstName: "Dark",
        LastName:  "Vador",
        Age:       30,
    }

    funk.Values(foo) // []interface{}{1, "Dark", "Vador", 30} (iteration order is not guaranteed)

funk.ForEach
............

Range over an iteratee (map, slice).

.. code-block:: go

    funk.ForEach([]int{1, 2, 3, 4}, func(x int) {
        fmt.Println(x)
    })

funk.ForEachRight
............

Range over an iteratee (map, slice) from the right.

.. code-block:: go

    results := []int{}

    funk.ForEachRight([]int{1, 2, 3, 4}, func(x int) {
        results = append(results, x)
    })

    fmt.Println(results) // []int{4, 3, 2, 1}

funk.Chunk
..........

Creates an array of elements split into groups with the length of the size.
If array can't be split evenly, the final chunk will be the remaining element.

.. code-block:: go

    funk.Chunk([]int{1, 2, 3, 4, 5}, 2) // [][]int{[]int{1, 2}, []int{3, 4}, []int{5}}

funk.FlattenDeep
................

Recursively flattens an array.

.. code-block:: go

    funk.FlattenDeep([][]int{[]int{1, 2}, []int{3, 4}}) // []int{1, 2, 3, 4}

funk.Uniq
.........

Creates an array with unique values.

.. code-block:: go

    funk.Uniq([]int{0, 1, 1, 2, 3, 0, 0, 12}) // []int{0, 1, 2, 3, 12}

see also, typesafe implementations: UniqInt_, UniqInt64_, UniqFloat32_, UniqFloat64_, UniqString_

.. _UniqFloat32: https://godoc.org/github.com/thoas/go-funk#UniqFloat32
.. _UniqFloat64: https://godoc.org/github.com/thoas/go-funk#UniqFloat64
.. _UniqInt: https://godoc.org/github.com/thoas/go-funk#UniqInt
.. _UniqInt64: https://godoc.org/github.com/thoas/go-funk#UniqInt64
.. _UniqString: https://godoc.org/github.com/thoas/go-funk#UniqString

funk.Drop
.........

Creates an array/slice with `n` elements dropped from the beginning.

.. code-block:: go

    funk.Drop([]int{0, 0, 0, 0}, 3) // []int{0}

see also, typesafe implementations: DropInt_, DropInt32_, DropInt64_, DropFloat32_, DropFloat64_, DropString_

.. _DropInt: https://godoc.org/github.com/thoas/go-funk#DropInt
.. _DropInt32: https://godoc.org/github.com/thoas/go-funk#DropInt64
.. _DropInt64: https://godoc.org/github.com/thoas/go-funk#DropInt64
.. _DropFloat32: https://godoc.org/github.com/thoas/go-funk#DropFloat32
.. _DropFloat64: https://godoc.org/github.com/thoas/go-funk#DropFloat64
.. _DropString: https://godoc.org/github.com/thoas/go-funk#DropString

funk.Initial
............

Gets all but the last element of array.

.. code-block:: go

    funk.Initial([]int{0, 1, 2, 3, 4}) // []int{0, 1, 2, 3}

funk.Tail
.........

Gets all but the first element of array.

.. code-block:: go

    funk.Tail([]int{0, 1, 2, 3, 4}) // []int{1, 2, 3, 4}

funk.Shuffle
............

Creates an array of shuffled values.

.. code-block:: go

    funk.Shuffle([]int{0, 1, 2, 3, 4}) // []int{2, 1, 3, 4, 0}


see also, typesafe implementations: ShuffleInt_, ShuffleInt64_, ShuffleFloat32_, ShuffleFloat64_, ShuffleString_

.. _ShuffleFloat32: https://godoc.org/github.com/thoas/go-funk#ShuffleFloat32
.. _ShuffleFloat64: https://godoc.org/github.com/thoas/go-funk#ShuffleFloat64
.. _ShuffleInt: https://godoc.org/github.com/thoas/go-funk#ShuffleInt
.. _ShuffleInt64: https://godoc.org/github.com/thoas/go-funk#ShuffleInt64
.. _ShuffleString: https://godoc.org/github.com/thoas/go-funk#ShuffleString

funk.Subtract
.............

Returns the subtraction between two collections. It preserve order.

.. code-block:: go

    funk.Subtract([]int{0, 1, 2, 3, 4}, []int{0, 4}) // []int{1, 2, 3}
    funk.Subtract([]int{0, 3, 2, 3, 4}, []int{0, 4}) // []int{3, 2, 3}


see also, typesafe implementations: SubtractString_

.. SubtractString: https://godoc.org/github.com/thoas/go-funk#SubtractString

funk.Sum
........

Computes the sum of the values in an array.

.. code-block:: go

    funk.Sum([]int{0, 1, 2, 3, 4}) // 10.0
    funk.Sum([]interface{}{0.5, 1, 2, 3, 4}) // 10.5

see also, typesafe implementations: SumInt_, SumInt64_, SumFloat32_, SumFloat64_

.. _SumFloat32: https://godoc.org/github.com/thoas/go-funk#SumFloat32
.. _SumFloat64: https://godoc.org/github.com/thoas/go-funk#SumFloat64
.. _SumInt: https://godoc.org/github.com/thoas/go-funk#SumInt
.. _SumInt64: https://godoc.org/github.com/thoas/go-funk#SumInt64

funk.Reverse
............

Transforms an array such that the first element will become the last, the second element
will become the second to last, etc.

.. code-block:: go

    funk.Reverse([]int{0, 1, 2, 3, 4}) // []int{4, 3, 2, 1, 0}

see also, typesafe implementations: ReverseInt_, ReverseInt64_, ReverseFloat32_, ReverseFloat64_, ReverseString_, ReverseStrings_

.. _ReverseFloat32: https://godoc.org/github.com/thoas/go-funk#ReverseFloat32
.. _ReverseFloat64: https://godoc.org/github.com/thoas/go-funk#ReverseFloat64
.. _ReverseInt: https://godoc.org/github.com/thoas/go-funk#ReverseInt
.. _ReverseInt64: https://godoc.org/github.com/thoas/go-funk#ReverseInt64
.. _ReverseString: https://godoc.org/github.com/thoas/go-funk#ReverseString
.. _ReverseStrings: https://godoc.org/github.com/thoas/go-funk#ReverseStrings

funk.SliceOf
............

Returns a slice based on an element.

.. code-block:: go

    funk.SliceOf(f) // will return a []*Foo{f}

funk.RandomInt
..............

Generates a random int, based on a min and max values.

.. code-block:: go

    funk.RandomInt(0, 100) // will be between 0 and 100

funk.RandomString
.................

Generates a random string with a fixed length.

.. code-block:: go

    funk.RandomString(4) // will be a string of 4 random characters

funk.Shard
..........

Generates a sharded string with a fixed length and depth.

.. code-block:: go

    funk.Shard("e89d66bdfdd4dd26b682cc77e23a86eb", 1, 2, false) // []string{"e", "8", "e89d66bdfdd4dd26b682cc77e23a86eb"}

    funk.Shard("e89d66bdfdd4dd26b682cc77e23a86eb", 2, 2, false) // []string{"e8", "9d", "e89d66bdfdd4dd26b682cc77e23a86eb"}

    funk.Shard("e89d66bdfdd4dd26b682cc77e23a86eb", 2, 3, true) // []string{"e8", "9d", "66", "bdfdd4dd26b682cc77e23a86eb"}

funk.Subset
.............

Returns true if a collection is a subset of another 

.. code-block:: go

    funk.Subset([]int{1, 2, 4}, []int{1, 2, 3, 4, 5}) // true
    funk.Subset([]string{"foo", "bar"},[]string{"foo", "bar", "hello", "bar", "hi"}) //true
   
    
Performance
-----------

``go-funk`` currently has an open issue about performance_, don't hesitate to participate in the discussion
to enhance the generic helpers implementations.

Let's stop beating around the bush, a typesafe implementation in pure Go of ``funk.Contains``, let's say for example:

.. code-block:: go

    func ContainsInt(s []int, e int) bool {
        for _, a := range s {
            if a == e {
                return true
            }
        }
        return false
    }

will always outperform an implementation based on reflect_ in terms of speed and allocs because of
how it's implemented in the language.

If you want a similarity, gorm_ will always be slower than sqlx_ (which is very low level btw) and will use more allocs.

You must not think generic helpers of ``go-funk`` as a replacement when you are dealing with performance in your codebase,
you should use typesafe implementations instead.

Contributing
------------

* Ping me on twitter `@thoas <https://twitter.com/thoas>`_ (DMs, mentions, whatever :))
* Fork the `project <https://github.com/thoas/go-funk>`_
* Fix `open issues <https://github.com/thoas/go-funk/issues>`_ or request new features

Don't hesitate ;)

Authors
-------

* Florent Messa
* Gilles Fabio
* Alexey Pokhozhaev
* Alexandre Nicolaie

.. _reflect: https://golang.org/pkg/reflect/
.. _lodash: https://lodash.com/
.. _performance: https://github.com/thoas/go-funk/issues/19
.. _gorm: https://github.com/jinzhu/gorm
.. _sqlx: https://github.com/jmoiron/sqlx
.. _godoc: https://godoc.org/github.com/thoas/go-funk
