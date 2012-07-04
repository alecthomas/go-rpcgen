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
