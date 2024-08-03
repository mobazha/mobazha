import Backbone, { Model, Collection } from 'backbone';
import { myAjax } from '../src/api/api';

if (!import.meta.env.VITE_APP) {
  Backbone.ajax = myAjax
}

export { Backbone, Model, Collection };
