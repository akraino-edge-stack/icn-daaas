### Kakfka based Inference APP

This is a sample app which demostrates how kafka based inteferencing can be done.
To understand how the app works, we shall explain the code and logic further below.

### main.py
This is the starting point of the app.
The main does the following things:
1. Invoke producer
2. Invoke consumer
3. Form queries
4. Collect the results of the queries.

```
NOTE : Invoking of the consumer is done by invoking a separate thread which runs the consume method. This consume method shall run infinitely.
Collecting the results shall be done by invoking the executeQuery method which reads from the global data structure-output_map.(output_map is explained in the consumer section.)
```

### Producer

* Since we wanted to write and test this app as standalone, we included the logic of producer inherently. The producer shall read a sample file - multithreading-metrics.json which shall have the colled metrics record. A sample record shall look like this :

```
{"labels":{"__name__":"go_gc_duration_seconds_count","endpoint":"http","instance":"10.42.1.92:8686","job":"prom-kafka-adapter","metrics_storage":"kafka_remote","namespace":"edge1","pod":"prom-kafka-adapter-5b9756c8d9-9vfgx","prometheus":"edge1/cp-prometheus-prometheus","prometheus_replica":"prometheus-cp-prometheus-prometheus-0","service":"prom-kafka-adapter"},"name":"go_gc_duration_seconds_count","timestamp":1572384034838,"value":12}
{"labels":{"__name__":"go_gc_duration_seconds_count","endpoint":"http","instance":"10.42.1.92:8686","job":"prom-kafka-adapter","metrics_storage":"kafka_remote","namespace":"edge1","pod":"prom-kafka-adapter-5b9756c8d9-9vfgx","prometheus":"edge1/cp-prometheus-prometheus","prometheus_replica":"prometheus-cp-prometheus-prometheus-0","service":"prom-kafka-adapter"},"name":"go_gc_duration_seconds_count","timestamp":1572384036838,"value":12}
```

* Each record is read as key value pair.
The key shall be the name of the metrics , for ex: *go_gc_duration_seconds_count* in the above case. We want to aggregate based on this key. Value shall be the entire record.

* This key value pair shall be passed to the *produce* method of the producer.
The produce method shall publish this data to a topic which can be configured at the producer level. In the sample app the topic we have is *metrics3*


### Consumer

* The consumer is responsible for the processing the message which arrives in the kafka queue under a topic, in our case this topic is *metrics3*.

* Another important thing that need to be kept in mind is the data-stucture - *output_map*. This map collects the records under each metrics. It clears based on the no. of records you want to collect. Once the threshold of the no. of records reach it pops out the oldest of the record it collected.

There are few other configs which are supported at the consumer level:
```
topic_name - the topic to which the producer writes and the consumer reads.
```
```
time_format - two time formats are upported, timestamp(by default) and iso
```
```
duration - Time period in the past for which you want to collect data. For eg, you want the data pertaining to the last 50 secs only. Most of the time, we might generate 1 record per second at the collectd level. So, we might end up with 50 records for a particular metrics.
```
```
no_of_records_wanted - This is kind of an optional parameter, where you can specify the no. of records to be collected under each metric name. In case you want 50 records coming at 1 record per sec , configure this as 50.
```