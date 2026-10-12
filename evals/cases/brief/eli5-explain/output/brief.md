# Saving the claims you picked

Now a reviewer can save just the claims they picked into one file. On average, reviewers do it 4.2 times per reviewer
per week [R].

## What it does

A claim is a note that asks for money. Reviewers check long lists of them. Before, they copied claims by hand, and it
took hours on long lists [#7].

Now they pick the claims they want, and they save them to a CSV file. A CSV file is a plain table that most computers
can open [#12].

The file has only the claims they picked [P1.c1]. It has only the columns they can see [P1.c2]. The list still works
while the file is made, even with more than 10,000 claims [P1.c3].

## How we know it works

Robot tests checked the change before it went in, and inspect said each part was met [P1]. The goal was 3 saves per
reviewer per week, and the count is 4.2 per reviewer per week [R].
