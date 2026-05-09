package main
import (
    "fmt"
    "os"
    "golang.org/x/crypto/bcrypt"
)
func main() {
    hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    fmt.Println(string(hash))
}
