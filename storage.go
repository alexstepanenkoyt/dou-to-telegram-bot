package main

import (
	"time"
)

type Storage interface {
	SetLastTimeCheckedUTC(category DouCategory) error
	GetLastTimeCheckedUTC(category DouCategory) time.Time
	SubscribeUser(category DouCategory, userId int, chatId int64, userName string) (bool, error)
	UnsubscribeUser(categoryId string, userId int) (bool, error)
	GetSubscriptionInfo(userId int) (SubscriptionInfo, error)
	GetAllSubscribers(categoryId string) ([]SubscriptionInfo, error)
}

type CategoryInfo struct {
	IDCategory      string `bson:"idCategory,omitempty"`
	NameCategory    string `bson:"nameCategory,omitempty"`
	LastTimeChecked string `bson:"lastTimeChecked,omitempty"`
}

type SubscriptionCategory struct {
	IDCategory   string `bson:"idCategory,omitempty"`
	NameCategory string `bson:"nameCategory,omitempty"`
}
type SubscriptionInfo struct {
	UserId        int                    `bson:"userId,omitempty"`
	ChatId        int64                  `bson:"chatId,omitempty"`
	UserName      string                 `bson:"userName,omitempty"`
	Subscriptions []SubscriptionCategory `bson:"subscriptions,omitempty"`
}
