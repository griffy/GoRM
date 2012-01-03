## GoRM

GoRM is an ORM for Go. It lets you map Go `struct`s to tables in a database. It's intended to be very lightweight, doing very little beyond what you really want. For example, when fetching data, instead of re-inventing a query syntax, we just delegate your query to the underlying database, so you can write the "where" clause of your SQL statements directly. This allows you to have more flexibility while giving you a convenience layer. But GoRM also has some smart defaults, for those times when complex queries aren't necessary.

### How do we use it?

To work with a database of your choosing, first import a library that implements exp/sql, like so:

    import _ "github.com/mattn/go-sqlite3"

And open a connection to it

	conn, _ := gorm.NewConnection("sqlite3", "./test.db")
	conn.Close() // you'll probably wanna close it at some point

Model a struct after a table in the db

	type Person struct {
		Id int64
		Name string
		Age int64
	}

Create a database session

    session, _ := conn.NewSession()

Create an object

	var someone Person
	someone.Name = "john"
	someone.Age = 20
	
And save it in two lines

	session.Update(&someone)
    session.Commit()

Update will either create a new row in your persons table or modify the matching one accordingly. However, the changes will not hit the database until you issue Commit. This is because GoRM is transaction-based.

If you'd rather type one line than two, you can do

    session.Save(&someone)

Finally, if you decide to commit and want a new transaction to further modify the database, you can issue

    session.Renew()

Fetch a single object

	var person1 Person
	session.Get(&person1, "id = ?", 3)

	var person2 Person
	session.Get(&person2, 3) // this is shorthand for the version above
	
	var person3 Person
	session.Get(&person3, "name = ?", "john") // more complex query
	
	var person4 Person
	session.Get(&person4, "name = ? and age < ?", "john", 88) // even more complex

Fetch multiple objects

	var bobs []Person
	err := session.GetAll(&bobs, "name = ?", "bob")

	var everyone []Person
	err := session.GetAll(&everyone) // omit "where" clause

Saving new and existing objects

	person2.Name = "Jack" // an already-existing person in the database, from the example above
	session.Save(&person2)
	session.Renew()

	var newGuy Person
	newGuy.Name = "that new guy"
	newGuy.Age = 27
	
	session.Save(&newGuy)
	// newGuy.Id is suddenly valid, and he's in the database now.

### Installing GoRM

Obviously [Go](http://golang.org/) should be installed. [The official installation directions](http://golang.org/doc/install.html) are recommended, rather than installing it through a package (such as homebrew).

### Known bugs

Also, at the moment, relationship-support is in the works, but not yet implemented.

All in all, it's not entirely ready for advanced use yet, but it's getting there.

### Etc

The idea came about in #go-nuts on irc.freenode.net... Namegduf and wrtp were instrumental in helping solidify the main principles, and I think wrtp came up with the name.

Feel free to send pull requests with cool features added :)
