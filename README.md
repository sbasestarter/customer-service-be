# customer-service-be

```bash
#
grpcui -plaintext -port 9999 localhost:12222

#
grpcurl -plaintext localhost:12222 list
grpcurl -plaintext localhost:12222 list CustomerTalkService
grpcurl -plaintext -rpc-header token:eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJJRCI6MTg0NTc0MDMwODQzMjY4MzAwOCwiVXNlck5hbWUiOiJ6anoiLCJleHAiOjE2NjkwOTUxODF9.3qqeSKcQxrr3CagAVQ79_sCSnBMmTM8u_k5jFHIjJUc localhost:12222 CustomerTalkService/Talk 
grpcurl -plaintext -d @ -rpc-header token:eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJJRCI6MTg0NTc0MDMwODQzMjY4MzAwOCwiVXNlck5hbWUiOiJ6anoiLCJleHAiOjE2NjkwOTUxODF9.3qqeSKcQxrr3CagAVQ79_sCSnBMmTM8u_k5jFHIjJUc localhost:12222 CustomerTalkService/Talk 


{"create":{"title":"hello,测试"}}
{"open":{"talkId":"635b8e888b053541c1d457ad"}}
{"message":{"seqId":"1","text":"zz,你好啊 1！"}}
{"close":{}}

grpcurl -plaintext -d @ -rpc-header token:eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJJRCI6MTg0NTc0MDMwODQzMjY4MzAwOCwiVXNlck5hbWUiOiJ6anoiLCJleHAiOjE2NjkwOTUxODF9.3qqeSKcQxrr3CagAVQ79_sCSnBMmTM8u_k5jFHIjJUc localhost:12222 ServiceTalkService/Service
{"attach":{"talkId":"635b8e888b053541c1d457ad"}}
{"message":{"talkId":"635b8e888b053541c1d457ad", "message":{"seqId":"101", "text":"你是WHO 4?"}}}
{"detach":{"talkId":"635b8e888b053541c1d457ad"}}
```
