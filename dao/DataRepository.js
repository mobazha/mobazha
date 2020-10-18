/**
 * Created by zhongpeng on 2017/11/7.
 */
import {
    AsyncStorage,
} from 'react-native';

/**
 * 数据持久化类
 */
export default class DataRepository {

    /**
     * 保存本地数据
     * @param key key
     * @param items value
     * @param callback  回调函数
     */
    saveLocalRepository(key, items, callback) {
        if (!items || !key)return;

        AsyncStorage.setItem(key, JSON.stringify(items), callback);

    }



    /**
     * 获取本地的数据
     * @param key
     * @returns {Promise}
     */
     fetchLocalRepository(key) {
        return new Promise((resolve, reject)=> {
            AsyncStorage.getItem(key, (error, result)=> {
                if (!error) {
                    try {
                        resolve(JSON.parse(result));
                    } catch (e) {
                        reject(e);
                        console.error(e);
                    }
                } else {
                    reject(error);
                    console.error(error);
                }
            })
        })
    }

    /**
     * 移除本地存存的key
     * @param key
     * @returns {*}
     */
    removeLocalRepository(key, callback) {

        if (!key)return;

        AsyncStorage.removeItem(key, callback);

    }

    /**
     * 移除本地存存的keys
     * @param keys
     * @returns {*}
     */
    removeMultiLocalRepository(keys, callback) {

        if (!keys)return;

        AsyncStorage.multiRemove(keys, callback);

    }

    /**
     * 获取数据
     * @param key 获取数据的key
     * @returns {Promise} 回调返回的Promise
     */
    fetchRepository(key) {
        return new Promise((resolve, reject)=> {
            this.fetchLocalRepository(key).then((wrapData)=> {
                if (wrapData) {
                    resolve(wrapData, true);
                } else {
                    this.fetchNetRepository(key).then((data)=> {
                        resolve(data);
                    }).catch((error)=> {
                        reject(error);
                    })
                }

            }).catch((error)=> {
                this.fetchNetRepository(key).then((data)=> {
                    resolve(data);
                }).catch((error=> {
                    reject(error);
                }))
            })
        })
    }

    /**
     * 获取网络的数据
     * @param key
     * @returns {Promise}
     */
    fetchNetRepository(key) {
        return new Promise((resolve, reject)=> {
                fetch(key)
                    .then((response)=>response.json())
                    .then((responseData)=> {
                        if (responseData) {
                            this.saveRepository(key, responseData);
                            resolve(responseData);
                        }else {
                            reject(new Error('responseData is null'));
                        }
                    })
                    .catch((error)=> {
                        reject(error);
                    })

        })
    }
}