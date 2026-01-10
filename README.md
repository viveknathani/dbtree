# dbtree

[![GitHub release](https://img.shields.io/github/v/release/viveknathani/dbtree?color=blue)](https://github.com/viveknathani/dbtree/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/viveknathani/dbtree)](https://goreportcard.com/report/github.com/viveknathani/dbtree)
[![License](https://img.shields.io/badge/license-Unlicense-blue.svg)](UNLICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/viveknathani/dbtree)](go.mod)
[![Tests](https://github.com/viveknathani/dbtree/workflows/test/badge.svg)](https://github.com/viveknathani/dbtree/actions)

A tool to visualize database schemas right inside your terminal. It's built for both humans and AI to get complete context of your database architecture.

<img src="demo.gif" width="700" alt="Demo">

<i>Works only with PostgreSQL for now.</i>

## features

- Visualize table relationships and foreign keys
- Multiple output formats: `text` or `json`
- Multiple output shapes: `tree`, `flat`, `chart`
- Shows columns, data types, and constraints (primary keys, foreign keys, unique constraints)
- Handles circular references

## installation

Linux/macOS:

```bash
curl -fsSL https://vivekn.dev/dbtree/install.sh | bash
```

Or from source:

```bash
git clone https://github.com/viveknathani/dbtree.git
cd dbtree
make build # binary will be created at `./bin/dbtree`
```

Or using go install:

```bash
go install github.com/viveknathani/dbtree/cmd/dbtree@latest
```

## usage

- `--conn` (required): PostgreSQL connection URL

  - Format: `postgres://username:password@host:port/database`
  - Example: `postgres://user:pass@localhost:5432/mydb`

- `--format` (optional): Output format

  - `text` (default): Human-readable text output
  - `json`: Structured JSON output

- `--shape` (optional): Visualization structure

  - `tree` (default): Hierarchical tree showing foreign key relationships
  - `flat`: Simple list of all tables
  - `chart`: ASCII diagram of the schema (works with text format only, also see more notes below for this shape)

- `--help`: Display help information

## examples

### Tree View (Text)

Shows tables in a hierarchical structure based on foreign key relationships:

```bash
dbtree --conn "postgres://user:pass@localhost:5432/mydb" --format text --shape tree
```

Output:

```
testdb
├── categories
│   ├── id ("integer") PRIMARY KEY
│   ├── name ("varchar(100)")
│   └── description ("text")
│   └── products
│       ├── id ("integer") PRIMARY KEY
│       ├── name ("varchar(200)")
│       ├── price ("numeric(10,2)")
│       ├── category_id ("integer") → categories.id
│       ├── stock ("integer")
│       └── created_at ("timestamp")
│       ├── order_items
│       │   ├── id ("integer") PRIMARY KEY
│       │   ├── order_id ("integer") → orders.id
│       │   ├── product_id ("integer") → products.id
│       │   ├── quantity ("integer")
│       │   └── price ("numeric(10,2)")
│       └── reviews
│           ├── id ("integer") PRIMARY KEY
│           ├── product_id ("integer") → products.id
│           ├── user_id ("integer") → users.id
│           ├── rating ("integer")
│           ├── comment ("text")
│           └── created_at ("timestamp")
└── users
    ├── id ("integer") PRIMARY KEY
    ├── username ("varchar(50)") UNIQUE
    ├── email ("varchar(100)") UNIQUE
    └── created_at ("timestamp")
    ├── addresses
    │   ├── id ("integer") PRIMARY KEY
    │   ├── user_id ("integer") → users.id
    │   ├── street ("varchar(200)")
    │   ├── city ("varchar(100)")
    │   ├── country ("varchar(100)")
    │   └── zip_code ("varchar(20)")
    ├── orders
    │   ├── id ("integer") PRIMARY KEY
    │   ├── user_id ("integer") → users.id
    │   ├── total_amount ("numeric(10,2)")
    │   ├── status ("varchar(50)")
    │   └── created_at ("timestamp")
    │   ├── order_items (see above)
    │   └── payments
    │       ├── id ("integer") PRIMARY KEY
    │       ├── order_id ("integer") → orders.id
    │       ├── payment_method ("varchar(50)")
    │       ├── amount ("numeric(10,2)")
    │       ├── status ("varchar(50)")
    │       └── created_at ("timestamp")
    └── reviews (see above)
```

### Flat List (Text)

Lists all tables alphabetically with their columns:

```bash
dbtree --conn "postgres://user:pass@localhost:5432/mydb" --format text --shape flat
```

Output:

```
Database: testdb
Tables: 8

addresses
  - id (integer) PRIMARY KEY
  - user_id (integer) → users.id
  - street (varchar(200))
  - city (varchar(100))
  - country (varchar(100))
  - zip_code (varchar(20))

categories
  - id (integer) PRIMARY KEY
  - name (varchar(100))
  - description (text)

order_items
  - id (integer) PRIMARY KEY
  - order_id (integer) → orders.id
  - product_id (integer) → products.id
  - quantity (integer)
  - price (numeric(10,2))

orders
  - id (integer) PRIMARY KEY
  - user_id (integer) → users.id
  - total_amount (numeric(10,2))
  - status (varchar(50))
  - created_at (timestamp)

payments
  - id (integer) PRIMARY KEY
  - order_id (integer) → orders.id
  - payment_method (varchar(50))
  - amount (numeric(10,2))
  - status (varchar(50))
  - created_at (timestamp)

products
  - id (integer) PRIMARY KEY
  - name (varchar(200))
  - price (numeric(10,2))
  - category_id (integer) → categories.id
  - stock (integer)
  - created_at (timestamp)

reviews
  - id (integer) PRIMARY KEY
  - product_id (integer) → products.id
  - user_id (integer) → users.id
  - rating (integer)
  - comment (text)
  - created_at (timestamp)

users
  - id (integer) PRIMARY KEY
  - username (varchar(50)) UNIQUE
  - email (varchar(100)) UNIQUE
  - created_at (timestamp)
```

### Chart View (ASCII Diagram)

Renders an ASCII chart diagram of the schema:

```bash
dbtree --conn "postgres://user:pass@localhost:5432/mydb" --format text --shape chart
```

<img src="./CHART_EXAMPLE.png">

<i>As of right now, this chart view looks ugly in a terminal and is probably not the best for an AI agent. However, when redirected to a text file, it looks decent enough for humans. I am open to feedback on how this can be improved.</i>

### JSON Output

Export schema information as structured JSON:

```bash
dbtree --conn "postgres://user:pass@localhost:5432/mydb" --format json --shape tree
```

Output:

```json
{
  "database": "mydb",
  "tables": [
    {
      "name": "users",
      "columns": [
        {
          "name": "id",
          "type": "integer",
          "constraint": "PRIMARY KEY"
        },
        {
          "name": "email",
          "type": "character varying",
          "constraint": "UNIQUE"
        },
        {
          "name": "name",
          "type": "character varying"
        }
      ],
      "children": [
        {
          "name": "posts",
          "columns": [
            {
              "name": "id",
              "type": "integer",
              "constraint": "PRIMARY KEY"
            },
            {
              "name": "user_id",
              "type": "integer",
              "reference": "users.id"
            }
          ]
        }
      ]
    }
  ]
}
```

## hacking

I am open to PRs for improving this project.

You will need a bunch of things:

1. make
2. Go (>=v1.25)
3. PostgreSQL

Some handy commands:

```bash
make build
make test
```

## license

[None](./UNLICENSE). Do whatever you want. Have fun and happy hacking!

<i>Built by [@viveknathani](https://vivekn.dev)</i>
