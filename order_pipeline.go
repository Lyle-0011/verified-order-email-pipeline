package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"net/url"
)

type orderStage string

const (
	stageAwaitingEmail orderStage = "awaiting_email_verification"
	stageCheckoutReady orderStage = "checkout_confirmed"
	stageFulfillment   orderStage = "fulfillment_queued"
)

type signup struct {
	OrderID             string     `json:"order_id"`
	Email               string     `json:"email"`
	Stage               orderStage `json:"stage"`
	VerificationMessage string     `json:"verification_message_id,omitempty"`
	ReceiptMessage      string     `json:"receipt_message_id,omitempty"`
	OrderUpdateMessage  string     `json:"order_update_message_id,omitempty"`
}

type orderPipeline struct {
	email  *infraiClient
	secret []byte
	public string
}

func (p *orderPipeline) start(ctx context.Context, orderID, email string) (signup, error) {
	token := p.sign(orderID, email)
	link := fmt.Sprintf("%s/verify?order_id=%s&email=%s&token=%s", p.public, url.QueryEscape(orderID), url.QueryEscape(email), token)
	sent, err := p.email.sendEmail(ctx, emailSendRequest{
		To:      email,
		Subject: "Verify your email to continue checkout",
		HTML:    fmt.Sprintf("<p>Verify this address for order <strong>%s</strong>.</p><p><a href=\"%s\">Verify email</a></p>", html.EscapeString(orderID), html.EscapeString(link)),
	}, "signup-verification-"+orderID)
	if err != nil {
		return signup{}, err
	}
	return signup{OrderID: orderID, Email: email, Stage: stageAwaitingEmail, VerificationMessage: sent.MessageID}, nil
}

func (p *orderPipeline) verify(ctx context.Context, current signup, token string) (signup, error) {
	if nextStage(current.Stage, hmac.Equal([]byte(token), []byte(p.sign(current.OrderID, current.Email)))) != stageCheckoutReady {
		return current, fmt.Errorf("verification token rejected")
	}
	if err := p.email.getEmail(ctx, current.VerificationMessage); err != nil {
		return current, fmt.Errorf("read verification handoff: %w", err)
	}
	current.Stage = stageCheckoutReady
	receipt, err := p.email.sendEmail(ctx, emailSendRequest{
		To: current.Email, Subject: "Receipt for order " + current.OrderID,
		HTML: fmt.Sprintf("<p>Payment recorded for order <strong>%s</strong>.</p>", html.EscapeString(current.OrderID)),
	}, "receipt-"+current.OrderID)
	if err != nil {
		return current, err
	}
	current.ReceiptMessage = receipt.MessageID
	current.Stage = stageFulfillment
	update, err := p.email.sendEmail(ctx, emailSendRequest{
		To: current.Email, Subject: "Order queued for fulfillment",
		HTML: fmt.Sprintf("<p>Order <strong>%s</strong> is queued for fulfillment.</p>", html.EscapeString(current.OrderID)),
	}, "fulfillment-update-"+current.OrderID)
	if err != nil {
		return current, err
	}
	current.OrderUpdateMessage = update.MessageID
	return current, nil
}

func nextStage(current orderStage, tokenValid bool) orderStage {
	if current == stageAwaitingEmail && tokenValid {
		return stageCheckoutReady
	}
	return current
}

func (p *orderPipeline) sign(orderID, email string) string {
	mac := hmac.New(sha256.New, p.secret)
	mac.Write([]byte(orderID + "\x00" + email))
	return hex.EncodeToString(mac.Sum(nil))
}
