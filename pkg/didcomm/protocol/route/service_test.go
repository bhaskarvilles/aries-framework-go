/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package route

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/service"
)

type updateResult struct {
	action string
	result string
}

func TestServiceNew(t *testing.T) {
	t.Run("test new service - success", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)
		require.Equal(t, Coordination, svc.Name())
	})

	t.Run("test new service name - failure", func(t *testing.T) {
		svc, err := New(&mockProvider{openStoreErr: errors.New("error opening the store")})
		require.Error(t, err)
		require.Contains(t, err.Error(), "open route coordination store")
		require.Nil(t, svc)
	})
}

func TestServiceAccept(t *testing.T) {
	s := &Service{}

	require.Equal(t, true, s.Accept(RequestMsgType))
	require.Equal(t, true, s.Accept(GrantMsgType))
	require.Equal(t, true, s.Accept(KeylistUpdateMsgType))
	require.Equal(t, true, s.Accept(KeylistUpdateResponseMsgType))
	require.Equal(t, false, s.Accept("unsupported msg type"))
}

func TestServiceHandleInbound(t *testing.T) {
	t.Run("test handle outbound ", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(&service.DIDCommMsg{Header: &service.Header{
			ID: msgID,
		}})
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})
}

func TestServiceHandleOutbound(t *testing.T) {
	t.Run("test handle outbound ", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		err = svc.HandleOutbound(nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not implemented")
	})
}

func TestServiceRequestMsg(t *testing.T) {
	t.Run("test service handle inbound request msg - success", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(generateRequestMsgPayload(t, msgID))
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})

	t.Run("test service handle request msg - success", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		msg := &service.DIDCommMsg{Payload: []byte("invalid json")}

		err = svc.handleRequest(msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "route request message unmarshal")
	})

	t.Run("test service handle request msg - verify outbound message", func(t *testing.T) {
		endpoint := "ws://agent.example.com"
		svc, err := New(&mockProvider{
			endpoint: endpoint,
			outbound: &mockOutbound{validateSend: func(msg interface{}) error {
				res, err := json.Marshal(msg)
				require.NoError(t, err)

				grant := &Grant{}
				err = json.Unmarshal(res, grant)
				require.NoError(t, err)

				require.Equal(t, endpoint, grant.Endpoint)
				require.Equal(t, 1, len(grant.RoutingKeys))

				return nil
			},
			},
		})
		require.NoError(t, err)

		msgID := randomID()

		err = svc.handleRequest(generateRequestMsgPayload(t, msgID))
		require.NoError(t, err)
	})
}

func TestServiceGrantMsg(t *testing.T) {
	t.Run("test service handle inbound grant msg - success", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(generateGrantMsgPayload(t, msgID))
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})

	t.Run("test service handle grant msg - success", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		msg := &service.DIDCommMsg{Payload: []byte("invalid json")}

		err = svc.handleGrant(msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "route grant message unmarshal")
	})
}

func TestServiceUpdateKeyListMsg(t *testing.T) {
	t.Run("test service handle inbound key list update msg - success", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(generateKeyUpdateListMsgPayload(t, msgID, []Update{{
			RecipientKey: "ABC",
			Action:       "add",
		}}))
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})

	t.Run("test service handle key list update msg - success", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		msg := &service.DIDCommMsg{Payload: []byte("invalid json")}

		err = svc.handleKeylistUpdate(msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "route key list update message unmarshal")
	})

	t.Run("test service handle request msg - verify outbound message", func(t *testing.T) {
		update := make(map[string]updateResult)
		update["ABC"] = updateResult{action: add, result: success}
		update["XYZ"] = updateResult{action: remove, result: serverError}
		update[""] = updateResult{action: add, result: serverError}

		svc, err := New(&mockProvider{

			outbound: &mockOutbound{validateSend: func(msg interface{}) error {
				res, err := json.Marshal(msg)
				require.NoError(t, err)

				updateRes := &KeylistUpdateResponse{}
				err = json.Unmarshal(res, updateRes)
				require.NoError(t, err)

				require.Equal(t, len(update), len(updateRes.Updated))

				for _, v := range updateRes.Updated {
					require.Equal(t, update[v.RecipientKey].action, v.Action)
					require.Equal(t, update[v.RecipientKey].result, v.Result)
				}

				return nil
			},
			},
		})
		require.NoError(t, err)

		msgID := randomID()

		var updates []Update
		for k, v := range update {
			updates = append(updates, Update{
				RecipientKey: k,
				Action:       v.action,
			})
		}

		err = svc.handleKeylistUpdate(generateKeyUpdateListMsgPayload(t, msgID, updates))
		require.NoError(t, err)
	})
}

func TestServiceKeylistUpdateResponseMsg(t *testing.T) {
	t.Run("test service handle inbound key list update response msg - success", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(generateKeylistUpdateResponseMsgPayload(t, msgID, []UpdateResponse{{
			RecipientKey: "ABC",
			Action:       "add",
			Result:       success,
		}}))
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})

	t.Run("test service handle key list update response msg - success", func(t *testing.T) {
		svc, err := New(&mockProvider{})
		require.NoError(t, err)

		msg := &service.DIDCommMsg{Payload: []byte("invalid json")}

		err = svc.handleKeylistUpdateResponse(msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "route keylist update response message unmarshal")
	})
}
