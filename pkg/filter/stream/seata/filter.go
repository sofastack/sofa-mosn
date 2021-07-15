package seata

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/opentrx/seata-golang/v2/pkg/apis"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"

	"mosn.io/api"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/protocol"
	mosnhttp "mosn.io/mosn/pkg/protocol/http"
	"mosn.io/mosn/pkg/seata"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"
)

const (
	XID      = "x_seata_xid"
	BranchID = "x_seata_branch_id"
)

type filter struct {
	conf              *v2.Seata
	transactionInfos  map[string]*v2.TransactionInfo
	tccResources      map[string]*v2.TCCResource
	receiveHandler    api.StreamReceiverFilterHandler
	sendHandler       api.StreamSenderFilterHandler
	transactionClient apis.TransactionManagerServiceClient
	resourceClient    apis.ResourceManagerServiceClient
}

func NewFilter(conf *v2.Seata) (*filter, error) {
	conn, err := grpc.Dial(conf.ServerAddressing,
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(conf.GetClientParameters()))
	if err != nil {
		return nil, err
	}

	transactionManagerClient:= apis.NewTransactionManagerServiceClient(conn)
	resourceManagerClient := apis.NewResourceManagerServiceClient(conn)

	f := &filter{
		conf:              conf,
		transactionInfos:  make(map[string]*v2.TransactionInfo),
		tccResources:      make(map[string]*v2.TCCResource),
		transactionClient: transactionManagerClient,
		resourceClient:    resourceManagerClient,
	}

	for _, ti := range conf.TransactionInfos {
		f.transactionInfos[ti.RequestPath] = ti
	}

	for _, r := range conf.TCCResources {
		f.tccResources[r.PrepareRequestPath] = r
	}

	return f, nil
}

func (f *filter) OnReceive(ctx context.Context, headers api.HeaderMap, buf buffer.IoBuffer, trailers api.HeaderMap) api.StreamFilterStatus {
	if f.receiveHandler.RequestInfo().Protocol() == protocol.HTTP1 {
		path, err := variable.GetString(ctx, types.VarPath)
		if err != nil {
			log.DefaultLogger.Errorf("failed to get path, err: %v", err)
			return api.StreamFiltertermination
		}
		method, err := variable.GetString(ctx, types.VarMethod)
		if err != nil {
			log.DefaultLogger.Errorf("failed to get method, err: %v", err)
			return api.StreamFiltertermination
		}

		if method != fasthttp.MethodPost {
			return api.StreamFilterContinue
		}

		transactionInfo, found := f.transactionInfos[strings.ToLower(path)]
		if found {
			return f.handleHttp1GlobalBegin(ctx, headers, buf, trailers, transactionInfo)
		}

		tccResource, exists := f.tccResources[strings.ToLower(path)]
		if exists {
			return f.handleHttp1BranchRegister(ctx, headers, buf, trailers, tccResource)
		}
	}
	return api.StreamFilterContinue
}

func (f *filter) Append(ctx context.Context, headers api.HeaderMap, buf buffer.IoBuffer, trailers api.HeaderMap) api.StreamFilterStatus {
	if f.receiveHandler.RequestInfo().Protocol() == protocol.HTTP1 {
		path, err := variable.GetString(ctx, types.VarPath)
		if err != nil {
			return api.StreamFilterContinue
		}
		method, err := variable.GetString(ctx, types.VarMethod)
		if err != nil {
			return api.StreamFilterContinue
		}

		if method != fasthttp.MethodPost {
			return api.StreamFilterContinue
		}

		_, found := f.transactionInfos[strings.ToLower(path)]
		if found {
			xid, _ := variable.GetString(ctx, XID)

			header, ok := headers.(mosnhttp.ResponseHeader)
			if ok {
				if header.StatusCode() == http.StatusOK {
					err = f.globalCommit(ctx, xid)
					if err != nil {
						log.DefaultLogger.Errorf(err.Error())
					}
				} else {
					err = f.globalRollback(ctx, xid)
					if err != nil {
						log.DefaultLogger.Errorf(err.Error())
					}
				}
			}
		}

		_, exists := f.tccResources[strings.ToLower(path)]
		if exists {
			xid, _ := variable.GetString(ctx, XID)
			branchIDStr, _ := variable.GetString(ctx, BranchID)
			branchID, _ := strconv.ParseInt(branchIDStr, 10, 64)

			header, ok := headers.(mosnhttp.ResponseHeader)
			if ok {
				if header.StatusCode() != http.StatusOK {
					err := f.branchReport(ctx, xid, branchID, apis.TCC, apis.PhaseOneFailed, nil)
					if err != nil {
						log.DefaultLogger.Errorf(err.Error())
					}
				}
			}
		}
	}

	return api.StreamFilterContinue
}

// SetReceiveFilterHandler sets decoder filter callbacks
func (f *filter) SetReceiveFilterHandler(handler api.StreamReceiverFilterHandler) {
	f.receiveHandler = handler
}

func (f *filter) SetSenderFilterHandler(handler api.StreamSenderFilterHandler) {
	f.sendHandler = handler
}

func (f *filter) OnDestroy() {

}

func (f *filter) handleHttp1GlobalBegin(ctx context.Context, headers api.HeaderMap, buf buffer.IoBuffer, trailers api.HeaderMap, transactionInfo *v2.TransactionInfo) api.StreamFilterStatus {

	// todo support transaction isolation level
	xid, err := f.globalBegin(ctx, transactionInfo.RequestPath, transactionInfo.Timeout)
	if err != nil {
		log.DefaultLogger.Errorf("failed to begin global transaction, transaction info: %v, err: %v", transactionInfo, err)
		return api.StreamFiltertermination
	}
	headers.Add(XID, xid)
	variable.Set(ctx, XID, xid)
	return api.StreamFilterContinue
}

func (f *filter) handleHttp1BranchRegister(ctx context.Context, headers api.HeaderMap, buf buffer.IoBuffer,
	trailers api.HeaderMap, tccResource *v2.TCCResource) api.StreamFilterStatus {
	xid, ok := headers.Get(XID)
	if !ok {
		log.DefaultLogger.Errorf("failed to get xid from request header")
		return api.StreamFiltertermination
	}

	requestContext := &seata.RequestContext{
		ActionContext: make(map[string]string),
		Headers:       protocol.CommonHeader{},
		Body:          buf.Clone(),
		Trailers:      protocol.CommonHeader{},
	}
	host, _:= variable.GetString(ctx, types.VarHost)
	requestContext.ActionContext[types.VarHost] = host
	requestContext.ActionContext[seata.CommitRequestPath] = tccResource.CommitRequestPath
	requestContext.ActionContext[seata.RollbackRequestPath] = tccResource.RollbackRequestPath
	queryString, _ := variable.GetString(ctx, types.VarQueryString)
	if queryString != "" {
		requestContext.ActionContext[types.VarQueryString] = queryString
	}
	if headers != nil {
		headers.Range(func(key, value string) bool {
			requestContext.Headers.Set(key, value)
			return true
		})
	}
	if trailers != nil {
		trailers.Range(func(key, value string) bool {
			requestContext.Trailers.Set(key, value)
			return true
		})
	}

	data, err := requestContext.Encode()
	if err != nil {
		log.DefaultLogger.Errorf("encode request context failed, request context: %v, err: %v", requestContext, err)
		return api.StreamFiltertermination
	}

	branchID, err := f.branchRegister(ctx, xid, tccResource.PrepareRequestPath, apis.TCC, data, "")
	if err != nil {
		log.DefaultLogger.Errorf("branch transaction register failed, xid: %s, err: %v", xid, err)
		return api.StreamFiltertermination
	}

	variable.Set(ctx, XID, xid)
	variable.Set(ctx, BranchID, strconv.FormatInt(branchID, 10))
	return api.StreamFilterContinue
}

func (f *filter) globalBegin(ctx context.Context, name string, timeout int32) (string, error) {
	request := &apis.GlobalBeginRequest{
		Addressing:      f.conf.Addressing,
		Timeout:         timeout,
		TransactionName: name,
	}
	resp, err := f.transactionClient.Begin(ctx, request)
	if err != nil {
		return "", err
	}
	if resp.ResultCode == apis.ResultCodeSuccess {
		return resp.XID, nil
	}
	return "", fmt.Errorf(resp.Message)
}

func (f *filter) globalCommit(ctx context.Context, xid string) error {
	var (
		err error
		status apis.GlobalSession_GlobalStatus
	)
	defer func() {
		variable.Set(ctx, XID, "")
	}()
	retry := f.conf.CommitRetryCount
	for retry > 0 {
		status, err = f.commit(ctx, xid)
		if err != nil {
			log.DefaultLogger.Errorf("failed to report global commit [%s],Retry Countdown: %d, reason: %s", xid, retry, err.Error())
		} else {
			break
		}
		retry--
		if retry == 0 {
			return errors.New("failed to report global commit")
		}
	}
	log.DefaultLogger.Infof("[%s] commit status: %s", xid, status.String())
	return nil
}

func (f *filter) globalRollback(ctx context.Context, xid string) error {
	var (
		err error
		status apis.GlobalSession_GlobalStatus
	)
	defer func() {
		variable.Set(ctx, XID, "")
	}()
	retry := f.conf.RollbackRetryCount
	for retry > 0 {
		status, err = f.rollback(ctx, xid)
		if err != nil {
			log.DefaultLogger.Errorf("failed to report global rollback [%s],Retry Countdown: %d, reason: %s", xid, retry, err.Error())
		} else {
			break
		}
		retry--
		if retry == 0 {
			return errors.New("failed to report global rollback")
		}
	}
	log.DefaultLogger.Infof("[%s] rollback status: %s", xid, status.String())
	return nil
}

func (f *filter) commit(ctx context.Context, xid string) (apis.GlobalSession_GlobalStatus, error) {
	request := &apis.GlobalCommitRequest{XID: xid}
	resp, err := f.transactionClient.Commit(ctx, request)
	if err != nil {
		return 0, err
	}
	if resp.ResultCode == apis.ResultCodeSuccess {
		return resp.GlobalStatus, nil
	}
	return 0, errors.New(resp.Message)
}

func (f *filter) rollback(ctx context.Context, xid string) (apis.GlobalSession_GlobalStatus, error) {
	request := &apis.GlobalRollbackRequest{XID: xid}
	resp, err := f.transactionClient.Rollback(ctx, request)
	if err != nil {
		return 0, err
	}
	if resp.ResultCode == apis.ResultCodeSuccess {
		return resp.GlobalStatus, nil
	}
	return 0, errors.New(resp.Message)
}

func (f *filter) branchRegister(ctx context.Context, xid string, resourceID string,
	branchType apis.BranchSession_BranchType, applicationData []byte, lockKeys string) (int64, error) {
	request := &apis.BranchRegisterRequest{
		Addressing:      f.conf.Addressing,
		XID:             xid,
		ResourceID:      resourceID,
		LockKey:         lockKeys,
		BranchType:      branchType,
		ApplicationData: applicationData,
	}
	resp, err := f.resourceClient.BranchRegister(ctx, request)
	if err != nil {
		return 0, err
	}
	if resp.ResultCode == apis.ResultCodeSuccess {
		return resp.BranchID, nil
	} else {
		return 0, fmt.Errorf(resp.Message)
	}
}

func (f *filter) branchReport(ctx context.Context, xid string, branchID int64,
branchType apis.BranchSession_BranchType, status apis.BranchSession_BranchStatus, applicationData []byte) error {
	request := &apis.BranchReportRequest{
		XID:             xid,
		BranchID:        branchID,
		BranchType:      branchType,
		BranchStatus:    status,
		ApplicationData: applicationData,
	}
	resp, err := f.resourceClient.BranchReport(ctx, request)
	if err != nil {
		return err
	}
	if resp.ResultCode == apis.ResultCodeFailed {
		return fmt.Errorf(resp.Message)
	}
	return nil
}