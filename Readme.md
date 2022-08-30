# Majic - MTG for Archer and Jonah Info Checker

This is Majic,
the MTG for Archer and Jonah Info Checker.

The goal of the project is to read a spreadsheet full of MTG cards and update each card’s price.

At the moment the code is just a proof-of-concept,
to see how well the API at scryfall.com works for card-price queries.
(It seems to work well!)

## Prerequisites

You will need to install the Go programming language.
This is simple to do.
Follow the instructions at [https://go.dev/](https://go.dev/).

## How to try the proof-of-concept

Once Go is installed, you can install the proof-of-concept program by running the command:

```sh
go install github.com/bobg/majic@latest
```

at a terminal prompt.

That will install a new command called `majic` that you can then run like this:

```sh
majic Giant Killer
```

This will report information about this card,
including the current price,
from the database at scryfall.

## Understand the code

The code is in [main.go](https://github.com/bobg/majic/blob/master/main.go).
It is well-commented to explain what’s going on.
