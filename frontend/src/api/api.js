import $ from 'jquery';
import axios from 'axios';

const api = axios.create({
  baseURL: import.meta.env.VITE_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

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
      // Handle unauthorized error
      router.push('/login'); // Redirect to login page
    }
    return Promise.reject(error);
  },
);

export function myGet(url, params = {}, config = {}) {
  const deferred = $.Deferred();

  const controller = new AbortController();
  api.get(url, { 
    params,
    ...config,
    signal: controller.signal
  })
    .then((response) => {
      deferred.resolve(response.data, response.statusText, response);
    })
    .catch((error) => {
      if (error.name === 'AbortError') {
        error.statusText = 'abort';
      }
      if (error.response) {
        error.responseJSON = error.response.data;
      }

      deferred.reject(error);
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
    headers: options.headers,
    signal: controller.signal
    // Other Axios options as needed
  };

  const deferred = $.Deferred();

  api(axiosConfig)
    .then((response) => {
      if (options.success) options.success(response.data);

      deferred.resolve(response.data, response.statusText, response);
    })
    .catch((error) => {
      if (error.name === 'AbortError') {
        error.statusText = 'abort';
      }
      if (error.response) {
        error.responseJSON = error.response.data;
      }

      if (options.error) options.error(error, error.statusText);

      deferred.reject(error);
    });

  deferred.abort = () => {
    controller.abort();
  };

  return deferred;
};

export default api;
