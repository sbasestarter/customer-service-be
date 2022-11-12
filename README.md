# customer-service-be

```bash
#
grpcui -plaintext -port 9999 localhost:12222

#
grpcurl -plaintext localhost:12222 list
grpcurl -plaintext localhost:12222 list CustomerTalkService
grpcurl.exe -plaintext -rpc-header token:eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxODUyOTc3MjU4MzA2NzMyMDMzLCJ1bmlxdWVfaWQiOjE4NTI5Nzc1NTEzOTIxMTI2NDAsInRva2VuX2xpdmVfZHVyYXRpb24iOjYwNDgwMDAwMDAwMDAwMCwiYXV0aGVudGljYXRvcl9tZXRob2RfZGF0YV9saXN0Ijp7ImFub255bW91cyI6IuWTiOWTiCJ9LCJleHAiOjE2Njg3NDcwNzR9.agt9XIqtEDBmvMCTLMTGIQJc4XeqFnZO3dnW3NCcGUc localhost:12222 CustomerTalkService/Talk 
grpcurl.exe -plaintext -d @ -rpc-header token:eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxODUyOTc3MjU4MzA2NzMyMDMzLCJ1bmlxdWVfaWQiOjE4NTI5Nzc1NTEzOTIxMTI2NDAsInRva2VuX2xpdmVfZHVyYXRpb24iOjYwNDgwMDAwMDAwMDAwMCwiYXV0aGVudGljYXRvcl9tZXRob2RfZGF0YV9saXN0Ijp7ImFub255bW91cyI6IuWTiOWTiCJ9LCJleHAiOjE2Njg3NDcwNzR9.agt9XIqtEDBmvMCTLMTGIQJc4XeqFnZO3dnW3NCcGUc localhost:12222 CustomerTalkService/Talk 

{"create":{"title":"hello,测试"}}
{"open":{"talkId":"636dd5fb823914978db65ac8"}}
{"message":{"seqId":"1","text":"zz,你好啊 3！"}}
{"close":{}}

grpcurl.exe -plaintext -d @ -rpc-header token:eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJ1bmlxdWVfaWQiOjE4NTI5Nzk2MjY2ODc1OTQ0OTYsInRva2VuX2xpdmVfZHVyYXRpb24iOjYwNDgwMDAwMDAwMDAwMCwiZXhwIjoxNjY4NzQ3NTY5fQ.rj6L2MOuuUz0Knr1ihqWT-KJnvPdLOgmhhP2wo9MyCQ localhost:12222 ServiceTalkService/Service
{"attach":{"talkId":"636dd5fb823914978db65ac8"}}
{"message":{"talkId":"636dd5fb823914978db65ac8", "message":{"seqId":"101", "text":"你是WHO 5?"}}}
{"detach":{"talkId":"636dd5fb823914978db65ac8"}}

```
