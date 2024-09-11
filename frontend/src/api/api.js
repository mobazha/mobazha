import $ from 'jquery';
import axios from 'axios';
import * as casdoor from '../utils/casdoor';

const api = axios.create({
  baseURL: import.meta.env.VITE_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

if (!import.meta.env.VITE_APP) {
  api.interceptors.request.use((config) => {
    // Add token to request headers if available
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  });
  
  api.interceptors.response.use(
    (response) => {
      return response;
    },
    (error) => {
      if (error.response && error.response.status === 401) {
        // Handle unauthorized error, redirect to login page
        window.location.href = casdoor.getSigninUrl();
      }
      return Promise.reject(error);
    },
  );
}

function handleError(deferred, error, options = {}) {
  const xhr = error.request || {};
  deferred.xhr = xhr;

  if (['AbortError', 'CanceledError'].includes(error.name)) {
    xhr.statusText = 'abort';
  }

  if (error.response) {
    xhr.responseJSON = error.response.data;

    if (options.error) options.error(xhr, error.response.statusText, error.response.data);

    deferred.reject(xhr, error.response.statusText, error.response.data);
  } else {
    if (options.error) options.error(xhr, xhr.statusText, error.message);

    deferred.reject(xhr, xhr.statusText, error.message);
  }
}

export function myGet(url, data = {}, config = {}) {
  const deferred = $.Deferred();

  const controller = new AbortController();
  api.get(url, { 
    params: data,
    ...config,
    signal: controller.signal
  })
    .then((response) => {
      deferred.resolve(response.data, "success", response.request);
    })
    .catch((error) => {
      handleError(deferred, error);
    });

  deferred.abort = () => {
    controller.abort();
  };

  return deferred;
}

export function myPost(url, data = {}, config = {}) {
  const deferred = $.Deferred();

  const controller = new AbortController();
  api.post(url, data, {
    ...config,
    signal: controller.signal
  })
    .then((response) => {
      deferred.resolve(response.data, "success", response.request);
    })
    .catch((error) => {
      handleError(deferred, error);
    });

  deferred.abort = () => {
    controller.abort();
  };

  return deferred;
}

export function myAjax(options) {
  const {
    url,
    type = 'GET',
    data,
    // Add other jQuery.ajax options as needed
  } = options;

  const controller = new AbortController();
  const axiosConfig = {
    method: type,
    url,
    data,
    headers: options.headers ? {...options.headers, 'Content-Type': 'application/json'} : {'Content-Type': 'application/json'},
    signal: controller.signal
    // Other Axios options as needed
  };

  const deferred = $.Deferred();

  api(axiosConfig)
    .then((response) => {
      if (options.success) options.success(response.data);

      deferred.resolve(response.data, "success", response.request);
    })
    .catch((error) => {
      handleError(deferred, error, options);
    });

  deferred.abort = () => {
    controller.abort();
  };

  return deferred;
};

export default api;
