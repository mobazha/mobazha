import Backbone from 'backbone';
import axios from 'axios';

// Axios interceptor for token check
axios.interceptors.request.use(config => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;

});

// Axios interceptor for handling 401 responses
axios.interceptors.response.use(response => {
  return response;
}, error => {
  if (error.response && error.response.status === 401) {
    router.push('/login'); // Redirect to login page
  }
  return Promise.reject(error);
});

Backbone.sync = function(method, model, options) {
  // Map Backbone methods to Axios methods
  const axiosMethodMap = {
    'create': 'post',
    'read': 'get',
    'update': 'put',
    'delete': 'delete'
  };

  const axiosMethod = axiosMethodMap[method.toLowerCase()];

  // Build Axios request configuration
  const config = {
    method: axiosMethod,
    url: options.url,
    data: model.toJSON(), // For create and update
    headers: options.headers
  };

  return axios(config)
    .then(response => {
      // Handle successful response
      if (options.success) {
        options.success(response.data);
      }
      return response.data;
    })
    .catch(error => {
      // Handle errors
      if (options.error) {
        options.error(error);
      }
      return error;
    });
};

export default Backbone;
