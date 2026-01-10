Output demo.gif

Set FontSize 20
Set Width 1400
Set Height 1000
Set Padding 20

Type "dbtree --conn postgresql://postgres:postgres@localhost:5434/testdb?sslmode=disable --shape tree"
Enter
Sleep 2s

Type "dbtree --conn postgresql://postgres:postgres@localhost:5434/testdb?sslmode=disable --shape flat"
Enter
Sleep 2s