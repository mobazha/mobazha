import $ from 'jquery';
import Backbone, { Model, Collection } from 'backbone';
import api from '../src/api/api';
import { CancelToken } from 'axios';

if (!import.meta.env.VITE_APP) {
  const source = CancelToken.source();

  Backbone.ajax = function (options) {
    const {
      url,
      type = 'GET',
      data,
      // Add other jQuery.ajax options as needed
    } = options;

    const axiosConfig = {
      method: type,
      url,
      data,
      headers: options.headers,
      cancelToken: source.token
      // Other Axios options as needed
    };

    const deferred = $.Deferred();

    api(axiosConfig)
      .then((response) => {
        if (options.success) options.success(response.data);

        deferred.resolve(response.data, response.statusText, response);
      })
      .catch((error) => {
        deferred.reject(error);
      });

    deferred.abort = (statusText) => {
      source.cancel(statusText);
    };

    return deferred;
  };
}

export { Backbone, Model, Collection };
