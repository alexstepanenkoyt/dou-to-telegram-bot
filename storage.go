package main

import (
	"time"
)

type Storage interface {
	SetLastTimeCheckedUTC(category DouCategory, exp string) error
	GetLastTimeCheckedUTC(category DouCategory, exp string) time.Time
	SubscribeUser(category DouCategory, exp string, userId int, chatId int64, userName string) (bool, error)
	UnsubscribeUser(categoryId string, userId int) (bool, error)
	GetSubscriptionInfo(userId int) (SubscriptionInfo, error)
	GetAllSubscribers(categoryName string, categoryId string, exp string) ([]SubscriptionInfo, error)
}

type CategoryInfo struct {
	IDCategory      string `bson:"idCategory,omitempty"`
	NameCategory    string `bson:"nameCategory,omitempty"`
	Experience      string `bson:"experience,omitempty"`
	LastTimeChecked string `bson:"lastTimeChecked,omitempty"`
}

type SubscriptionCategory struct {
	IDCategory   string `bson:"idCategory,omitempty"`
	NameCategory string `bson:"nameCategory,omitempty"`
	Experience   string `bson:"experience,omitempty"`
}
type SubscriptionInfo struct {
	UserId        int                    `bson:"userId,omitempty"`
	ChatId        int64                  `bson:"chatId,omitempty"`
	UserName      string                 `bson:"userName,omitempty"`
	CreateDate    string                 `bson:"createDate,omitempty"`
	Subscriptions []SubscriptionCategory `bson:"subscriptions,omitempty"`
}
