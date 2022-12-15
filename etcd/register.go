package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Register struct {
	ServeAddrs  []string
	DialTimeout int // 租期

	closeCh     chan struct{}
	leasesID    clientv3.LeaseID                        // 租赁ID，
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse // 客户端和etcd通信的通道

	srvInfo  Server
	interval int64            // 心跳周期
	cli      *clientv3.Client // 连接etcd的客户端
	logger   *logrus.Logger
}

// NewRegister create a register based on etcd
func NewRegister(serverAdds []string, logger *logrus.Logger) *Register {
	return &Register{
		ServeAddrs:  serverAdds,
		DialTimeout: 3,
		logger:      logger,
	}
}

// Register 注册服务，申请租赁凭证，并向etcd挂载一个节点
func (r *Register) Register(srvInfo Server, interval int64) (chan<- struct{}, error) {
	var err error

	// 检查一下
	if strings.Split(srvInfo.Addr, ":")[0] == "" {
		return nil, errors.New("invalid ip address")
	}

	if r.cli, err = clientv3.New(clientv3.Config{
		Endpoints:   r.ServeAddrs,
		DialTimeout: time.Duration(r.DialTimeout) * time.Second,
	}); err != nil {
		return nil, err
	}

	r.srvInfo = srvInfo
	r.interval = interval
	// 前边都是注册前的数据准备, 这个方法才是与etcd沟通
	if err = r.register(); err != nil {
		return nil, err
	}

	r.closeCh = make(chan struct{})

	go r.heatBeat()

	return r.closeCh, nil
}

func (r *Register) register() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.DialTimeout)*time.Second)
	defer cancel()

	// 通过etcd客户端租赁 生成租赁合同哦
	// 仅仅是获取租赁凭证，此时服务并没有挂在上去
	// leaseResp 租赁凭证
	leaseResp, err := r.cli.Grant(ctx, r.interval)
	if err != nil {
		return err
	}

	r.leasesID = leaseResp.ID

	// 租赁是会过期的
	if r.keepAliveCh, err = r.cli.KeepAlive(context.Background(), r.leasesID); err != nil {
		return err
	}

	data, err := json.Marshal(r.srvInfo)
	if err != nil {
		return err
	}

	// 这个是真正的挂载服务
	_, err = r.cli.Put(context.Background(), BuildRegisterPath(r.srvInfo), string(data), clientv3.WithLease(r.leasesID))

	return err
}

// Stop to stop register
func (r *Register) Stop() {
	r.closeCh <- struct{}{}
}

// unregister 删除节点，卸载服务
func (r *Register) unregister() error {
	_, err := r.cli.Delete(context.Background(), BuildRegisterPath(r.srvInfo))
	return err
}

func (r *Register) heatBeat() {
	ticker := time.NewTicker(time.Duration(r.interval) * time.Second)

	for {
		select {
		case <-r.closeCh:
			// 卸载服务节点
			if err := r.unregister(); err != nil {
				r.logger.Error("unregister failed, error: ", err)
			}

			// 撤销续约
			if _, err := r.cli.Revoke(context.Background(), r.leasesID); err != nil {
				r.logger.Error("revoke failed, error: ", err)
			}
		case res := <-r.keepAliveCh:
			// 取出来空说明,etcd把keepAliveCh关了
			if res == nil {
				if err := r.register(); err != nil {
					r.logger.Error("register failed, error: ", err)
				}
			}
		case <-ticker.C:
			if r.keepAliveCh == nil {
				if err := r.register(); err != nil {
					r.logger.Error("register failed, error: ", err)
				}
			}
		}
	}
}

func (r *Register) UpdateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		weightstr := req.URL.Query().Get("weight")
		weight, err := strconv.Atoi(weightstr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		var update = func() error {
			r.srvInfo.Weight = int64(weight)
			data, err := json.Marshal(r.srvInfo)
			if err != nil {
				return err
			}

			_, err = r.cli.Put(context.Background(), BuildRegisterPath(r.srvInfo), string(data), clientv3.WithLease(r.leasesID))
			return err
		}

		if err := update(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		_, _ = w.Write([]byte("update server weight success"))
	}
}

func (r *Register) GetServerInfo() (Server, error) {
	resp, err := r.cli.Get(context.Background(), BuildRegisterPath(r.srvInfo))
	if err != nil {
		return r.srvInfo, err
	}

	server := Server{}
	if resp.Count >= 1 {
		if err := json.Unmarshal(resp.Kvs[0].Value, &server); err != nil {
			return server, err
		}
	}

	return server, err
}
