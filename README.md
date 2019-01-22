

### 使用配置
```
<?xml version="1.0" encoding="UTF-8" ?>
<config>
    <!--服务器地址:端口-->
    <address>ip:port</address>
    <!--修改成报告存放目录-->
    <repertory>/data/server/upp/upload</repertory>
    <!--允许上传文件minetype-->
    <allow>
        <contentType>image/png</contentType>
        <contentType>image/jpeg</contentType>
        <contentType>application/pdf</contentType>
    </allow>
</config>
```

### 使用
```
./bin/upp --config=config.xml
```

### support
* qq group 233415606
