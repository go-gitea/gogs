go-funk changelog
=================

0.1 (2017-01-18)
----------------

Changes can be seen [here](https://github.com/thoas/go-funk/compare/73b8ae1f6443c9d4acbdc612bbb2ca804bb39b1d...master)

* Better test suite
* Better documentation
* Add typesafe implementations:

  * ``Contains``
  * ``Sum``
  * ``Reverse``
  * ``IndexOf``
  * ``Uniq``
  * ``Shuffle``
* Add benchmarks

  * ``Contains``
  * ``Uniq``
  * ``Sum``
* Fix ``redirectValue`` when using a circular reference
* Add ``Sum`` generic implementation which computes the sum of values in an array
* Add ``Tail`` generic implementation to retrieve all but the first element of array
* Add ``Initial`` generic implementation to retrieve all but the last element of array
* Add ``Last`` generic implementation to retrieve the last element of an array
* Add ``Head`` generic implementation to retrieve the first element of an array
