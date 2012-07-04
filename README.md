# Generates RPC server and client stubs for Go

This utility generates server and client RPC stubs from a Go interface. These
stubs eliminate a large amount of the boilerplate code required for creating
convenient, clean RPC interfaces.

## Limitations

The source type must:

1. Be an `interface` (`struct`s are not currently supported).
2. Name all of its return values.
3. Return an `error` as the last value in its return type.

## Generating the stubs

If you had a file `arith.go` containing this interface:

    package arith

    type Arith interface {
  	  Add(a, b int) (result int, err error)
    }

The following command will generate stubs for the interface:

    go-rpcgen --source=arith.go --type=Arith

That will generate a file named `arithrpc.go` (by default) containing two
types, `ArithService` and `ArithClient`, that can be used with the Go RPC
system, and as a client for the system, respectively.

The generated code will look something like this:

    package arith
    
    import (
    	"net/rpc"
    )
    
    type ArithService struct {
    	impl Arith
    }
    
    func NewArithService(impl Arith) *ArithService {
    	return &ArithService{impl}
    }
    
    type AddRequest struct {
    	A, B int
    }
    
    type AddResponse struct {
    	Result int
    }
    
    func (s *ArithService) Add(request *AddRequest, response *AddResponse) (err error) {
    	response.Result, err = s.impl.Add(request.A, request.B)
    	return
    }
    
    type ArithClient struct {
    	client *rpc.Client
    	service string
    }
    
    func NewArithClient(client *rpc.Client, service string) *ArithClient {
    	return &ArithClient{client, service}
    }
    
    func (_c *ArithClient) Add(a, b int) (result int, err error) {
    	request := &AddRequest{a, b}
    	response := &AddResponse{}
    	err = _c.client.Call(_c.service + ".Add", request, response)
    	return response.Result, err
    }
