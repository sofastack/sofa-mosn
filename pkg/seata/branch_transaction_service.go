package seata

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/go-resty/resty/v2"
	"github.com/opentrx/seata-golang/v2/pkg/apis"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/protocol"
	"mosn.io/mosn/pkg/types"
	"mosn.io/pkg/buffer"
	"mosn.io/pkg/utils"
)

const(
	CommitRequestPath = "tcc_commit_request_path"
	RollbackRequestPath = "tcc_rollback_request_path"
)

func Init(branchTransactionServicePort int, kaep keepalive.EnforcementPolicy, kasp keepalive.ServerParameters) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", branchTransactionServicePort))
	if err != nil {
		log.DefaultLogger.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp))
	apis.RegisterBranchTransactionServiceServer(s, &BranchTransactionService{})

	utils.GoWithRecover(func() {
		if err := s.Serve(lis); err != nil {
			log.DefaultLogger.Fatalf("failed to serve: %v", err)
		}
	}, nil)
}

// BranchTransactionService when global transaction commit, transaction coordinator callback to commit branch transaction,
// when global transaction rollback, transaction coordinator callback to rollback branch transaction.
type BranchTransactionService struct {

}

// BranchCommit commit branch transaction
func (svc *BranchTransactionService) BranchCommit(ctx context.Context, request *apis.BranchCommitRequest) (*apis.BranchCommitResponse, error) {
	requestContext := &RequestContext{
		ActionContext: make(map[string]string),
		Headers:       protocol.CommonHeader{},
		Body:          buffer.NewIoBuffer(0),
		Trailers:      protocol.CommonHeader{},
	}

	requestContext.Decode(request.ApplicationData)


	resp, err := doHttp1Request(requestContext, true)
	if err != nil {
		log.DefaultLogger.Errorf("commit failed, err: %v", err)
		return &apis.BranchCommitResponse{
			ResultCode: apis.ResultCodeFailed,
			Message:    err.Error(),
		}, nil
	}

	if resp.StatusCode() == http.StatusOK {
		return &apis.BranchCommitResponse{
			ResultCode:   apis.ResultCodeSuccess,
			XID:          request.XID,
			BranchID:     request.BranchID,
			BranchStatus: apis.PhaseTwoCommitted,
		}, nil
	} else {
		return &apis.BranchCommitResponse{
			ResultCode:   apis.ResultCodeSuccess,
			XID:          request.XID,
			BranchID:     request.BranchID,
			BranchStatus: apis.PhaseTwoCommitFailedRetryable,
		}, nil
	}
}

// BranchRollback rollback branch transaction
func (svc *BranchTransactionService) BranchRollback(ctx context.Context, request *apis.BranchRollbackRequest) (*apis.BranchRollbackResponse, error) {
	requestContext := &RequestContext{
		ActionContext: make(map[string]string),
		Headers:       protocol.CommonHeader{},
		Body:          buffer.NewIoBuffer(0),
		Trailers:      protocol.CommonHeader{},
	}

	requestContext.Decode(request.ApplicationData)

	resp, err := doHttp1Request(requestContext, false)
	if err != nil {
		log.DefaultLogger.Errorf("commit failed, err: %v", err)
		return &apis.BranchRollbackResponse{
			ResultCode: apis.ResultCodeFailed,
			Message:    err.Error(),
		}, nil
	}

	if resp.StatusCode() == http.StatusOK {
		return &apis.BranchRollbackResponse{
			ResultCode:   apis.ResultCodeSuccess,
			XID:          request.XID,
			BranchID:     request.BranchID,
			BranchStatus: apis.PhaseTwoRolledBack,
		}, nil
	} else {
		return &apis.BranchRollbackResponse{
			ResultCode:   apis.ResultCodeSuccess,
			XID:          request.XID,
			BranchID:     request.BranchID,
			BranchStatus: apis.PhaseTwoRollbackFailedRetryable,
		}, nil
	}
}

func doHttp1Request(requestContext *RequestContext, commit bool) (*resty.Response, error) {
	var (
		host string
		path string
		queryString string
	)
	host = requestContext.ActionContext[types.VarHost]
	if commit {
		path = requestContext.ActionContext[CommitRequestPath]
	} else {
		path = requestContext.ActionContext[RollbackRequestPath]
	}

	u := url.URL{
		Scheme: "http",
		Path: path,
		Host: host,
	}
	queryString, ok := requestContext.ActionContext[types.VarQueryString]
	if ok {
		u.RawQuery = queryString
	}

	client := resty.New()
	request := client.R()
	requestContext.Headers.Range(func(key, value string) bool {
		request.SetHeader(key, value)
		return true
	})
	request.SetBody(requestContext.Body.Bytes())
	return request.Post(u.String())
}
