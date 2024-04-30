package main

import (
	"fmt"
	"io"
	"time"

	"github.com/KScaesar/Artifex"
)

func main() {
	Artifex.SetDefaultLogger(Artifex.NewLogger(false, Artifex.LogLevelDebug))

	routeDelimiter := "/"
	mux := Artifex.NewMux(routeDelimiter)

	mux.ErrorHandler(Artifex.UsePrintResult())

	// Note:
	// Before registering handler, middleware must be defined;
	// otherwise, the handler won't be able to use middleware.
	mux.Middleware(
		Artifex.UseExclude([]string{"RegisterUser"}),
		Artifex.UseLogger(false, false).PreMiddleware(),
		Artifex.UseHowMuchTime(),
		func(next Artifex.HandleFunc) Artifex.HandleFunc {
			return func(message *Artifex.Message, dep any) error {
				logger := Artifex.CtxGetLogger(message.Ctx)
				logger.Info(">>>>>> recv %q <<<<<<", message.Subject)
				return next(message, dep)
			}
		},
		Artifex.UseRecover(),
	)

	// When a subject cannot be found, execute the 'Default'
	mux.DefaultHandler(Artifex.UsePrintDetail())

	v1 := mux.Group("v1/").Middleware(HandleAuth().PreMiddleware())

	v1.Handler("Hello/{user}", Hello)

	db := make(map[string]any)
	v1.Handler("UpdatedProductPrice/{brand}", UpdatedProductPrice(db))

	// Endpoints:
	// [Artifex] subject=".*"                                f="main.main.UsePrintDetail.func9"
	// [Artifex] subject="v1/Hello/{user}"                   f="main.Hello"
	// [Artifex] subject="v1/UpdatedProductPrice/{brand}"    f="main.main.UpdatedProductPrice.func12"
	mux.Endpoints(func(subject, fn string) { fmt.Printf("[Artifex] subject=%-35q f=%q\n", subject, fn) })

	intervalSecond := 2
	Listen(mux, intervalSecond)
}

func Listen(mux *Artifex.Mux, second int) {
	adapter := NewAdapter(second)
	fmt.Printf("wait %v seconds\n\n", second)
	for {
		message, err := adapter.Recv()
		if err != nil {
			return
		}

		mux.HandleMessage(message, nil)
	}
}

// adapter

func NewAdapter(second int) *Adapter {
	mq := make(chan *Artifex.Message, 1)
	size := len(messages)

	go func() {
		ticker := time.NewTicker(time.Duration(second) * time.Second)
		defer ticker.Stop()

		cursor := 0
		for {
			select {
			case <-ticker.C:
				if cursor >= size {
					close(mq)
					return
				}
				mq <- messages[cursor]
				cursor++
			}
		}
	}()

	return &Adapter{mq: mq}
}

type Adapter struct {
	mq chan *Artifex.Message
}

func (adp *Adapter) Recv() (msg *Artifex.Message, err error) {
	defer func() {
		if err != nil {
			fmt.Printf("recv message fail\n")
		}
	}()

	message, ok := <-adp.mq
	if !ok {
		return nil, io.EOF
	}
	return message, nil

}

// message

var messages = createMessages()

func createMessages() (messages []*Artifex.Message) {
	type param struct {
		Subject string
		Bytes   []byte
	}
	params := []param{
		{
			Subject: "RegisterUser",
			Bytes:   []byte(`{"user_id": "123456", "username": "john_doe", "email": "john.doe@example.com", "age": 30, "country": "United States"}`),
		},
		{
			Subject: "v1/Hello/ff1017",
			Bytes:   []byte("world"),
		},
		{
			Subject: "UpdatedUser",
			Bytes:   []byte(`{"user_id": "789012", "username": "jane_smith", "email": "jane.smith@example.com", "age": 25, "country": "Canada"}`),
		},
		{
			Subject: "v1/UpdatedProductPrice/Samsung",
			Bytes:   []byte(`{"product_id": "67890", "name": "Samsung Galaxy Watch 4", "price": 349, "brand": "Samsung", "category": "Wearable Technology"}`),
		},
		{
			Subject: "UpdateLocation",
			Bytes:   []byte(`{"location_id": "002", "name": "Eiffel Tower", "city": "Paris", "country": "France", "latitude": 48.8584, "longitude": 2.2945}`),
		},
		{
			Subject: "CreatedOrder",
			Bytes:   []byte(`{"order_id": "ABC123", "customer_name": "John Smith", "total_amount": 150.75, "items": ["T-shirt", "Jeans", "Sneakers"]}`),
		},
	}

	for i := range params {
		message := Artifex.GetMessage()
		message.Subject = params[i].Subject
		message.Bytes = params[i].Bytes
		messages = append(messages, message)
	}
	return messages
}

// handler

func HandleAuth() Artifex.HandleFunc {
	return func(message *Artifex.Message, dep any) error {
		Artifex.CtxGetLogger(message.Ctx).
			Info("Middleware: Auth ok")
		return nil
	}
}

func Hello(message *Artifex.Message, dep any) error {
	Artifex.CtxGetLogger(message.Ctx).
		Info("Hello: body=%v user=%v\n", string(message.Bytes), message.RouteParam.Get("user"))
	return nil
}

func UpdatedProductPrice(db map[string]any) Artifex.HandleFunc {
	return func(message *Artifex.Message, dep any) error {
		brand := message.RouteParam.Str("brand")
		db[brand] = message.Bytes

		Artifex.CtxGetLogger(message.Ctx).
			Info("UpdatedProductPrice: saved db: brand=%v\n", brand)
		return nil
	}
}
