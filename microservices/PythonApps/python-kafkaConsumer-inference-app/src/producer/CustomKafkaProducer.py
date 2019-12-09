#
# -------------------------------------------------------------------------
#   Copyright (c) 2019 Intel Corporation Intellectual Property
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
#
# -------------------------------------------------------------------------
#

import logging
from confluent_kafka import Producer
import traceback

logging.basicConfig(format='%(asctime)s::%(process)d::%(levelname)s::%(message)s', level=logging.INFO, datefmt='%d-%b-%y %H:%M:%S')


class CustomKafkaProducer:
    def __init__(self):
        self.topic_name = "metrics3"
        #self.topic_name = "adatopic1"
        conf = {'bootstrap.servers': 'kafka-cluster-kafka-bootstrap:9092'
                }
        self.producer = Producer(**conf)


    def produce(self, kafka_msg, kafka_key):
        try:
            self.producer.produce(topic=self.topic_name,
                              value=kafka_msg,
                              key=kafka_key,
                              callback=lambda err, msg: self.on_delivery(err, msg)
            )
            self.producer.flush()

        except Exception as e:
            #print("Error during producing to kafka topic. Stacktrace is %s",e)
            logging.error("Error during producing to kafka topic.")
            traceback.print_exc()


    def on_delivery(self, err, msg):
        if err:
            print("Message failed delivery, error: %s", err)
            logging.error('%s raised an error', err)
        else:
            logging.info("Message delivered to %s on partition %s",
                        msg.topic(), msg.partition())