-- Users table
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Categories table
CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT
);

-- Products table
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    category_id INTEGER REFERENCES categories(id),
    stock INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Addresses table
CREATE TABLE addresses (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    street VARCHAR(200),
    city VARCHAR(100),
    country VARCHAR(100),
    zip_code VARCHAR(20)
);

-- Orders table
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    total_amount DECIMAL(10, 2),
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Order items table
CREATE TABLE order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES orders(id) ON DELETE CASCADE,
    product_id INTEGER REFERENCES products(id),
    quantity INTEGER NOT NULL,
    price DECIMAL(10, 2) NOT NULL
);

-- Reviews table
CREATE TABLE reviews (
    id SERIAL PRIMARY KEY,
    product_id INTEGER REFERENCES products(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id),
    rating INTEGER CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Payments table
CREATE TABLE payments (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES orders(id),
    payment_method VARCHAR(50),
    amount DECIMAL(10, 2),
    status VARCHAR(50) DEFAULT 'completed',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample data
INSERT INTO users (username, email) VALUES
    ('john_doe', 'john@example.com'),
    ('jane_smith', 'jane@example.com'),
    ('bob_wilson', 'bob@example.com');

INSERT INTO categories (name, description) VALUES
    ('Electronics', 'Electronic devices and gadgets'),
    ('Clothing', 'Apparel and fashion items'),
    ('Books', 'Physical and digital books');

INSERT INTO products (name, price, category_id, stock) VALUES
    ('Laptop', 999.99, 1, 10),
    ('Smartphone', 599.99, 1, 25),
    ('T-Shirt', 19.99, 2, 100),
    ('Jeans', 49.99, 2, 50),
    ('Programming Book', 39.99, 3, 30);

INSERT INTO addresses (user_id, street, city, country, zip_code) VALUES
    (1, '123 Main St', 'New York', 'USA', '10001'),
    (2, '456 Oak Ave', 'Los Angeles', 'USA', '90001'),
    (3, '789 Pine Rd', 'Chicago', 'USA', '60601');

INSERT INTO orders (user_id, total_amount, status) VALUES
    (1, 1019.98, 'completed'),
    (2, 599.99, 'pending'),
    (3, 69.98, 'completed');

INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
    (1, 1, 1, 999.99),
    (1, 3, 1, 19.99),
    (2, 2, 1, 599.99),
    (3, 3, 2, 19.99),
    (3, 5, 1, 39.99);

INSERT INTO reviews (product_id, user_id, rating, comment) VALUES
    (1, 1, 5, 'Great laptop!'),
    (2, 2, 4, 'Good phone, battery could be better'),
    (3, 3, 5, 'Perfect fit!');

INSERT INTO payments (order_id, payment_method, amount, status) VALUES
    (1, 'credit_card', 1019.98, 'completed'),
    (3, 'paypal', 69.98, 'completed');
