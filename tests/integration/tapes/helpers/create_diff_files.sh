#!/bin/bash
# Create test files for diff popup visual test.
cat > /tmp/old.go <<'EOF'
package main

import "fmt"

func main() {
	fmt.Println("hello")
	fmt.Println("world")
}
EOF
cat > /tmp/new.go <<'EOF'
package main

import "fmt"

func main() {
	fmt.Println("hello, world")
	fmt.Println("goodbye")
	fmt.Println("new line")
}
EOF
