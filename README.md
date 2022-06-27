# pulumi-go-provider

A framework for building Go Providers for Pulumi

## Basic desired use case

```go
func main() {
    providers.Run("xyz", semver.MustParse("1.2.3"),
        providers.Resources(resource1{}, resource2),
        providers.Types(type1{}, type2{}),
        providers.Components(comp1{}))
}
```

## Expected internal architecture

```go
// In the provider package
func Resources(resources ...Resource) []Resource


type CRUD interface {

}

type Resource {}

func serialize(package string, t []interface{}) -> schema.SchemaSpec {
    ResourceTypeSpec {
        token: "command:index:Command",
    }
}

// In user space
type Commmand struct {
    Resource

    text string
    port int
    connection Connection
}

type Connection struct {
    port int
    username string
}

// We want
serialize("command", []interface{}{Command{}})
// To output something like
schema.PackageSpec {
	Resources: map[string]schema.ResourceSpec{
		"command:index:Command": schema.ResourceSpec{
			properties: []schema.PropertySpec{
				{Name: "text", Type: "string"},
				{Name: "port", Type: "int"},
				{Name: "connection", $Ref: "#/types/command:index:Connection"},
			}
		},
	}
	Types: map[string]schema.ObjectSpec{
		"command:index:Connetion": schema.ObjectSpec{
			properties: []schema.PropertySpec{
				{Name: "port", Type: "int"}
				{Name: "username", Type: "string"}
			}
		},
	}
}
```
