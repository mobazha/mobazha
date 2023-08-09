import DOMPurify from 'dompurify'
import { processMessage } from './ChatMessage';
import app from '../../app';
import BaseModel from '../BaseModel';

export default class extends BaseModel {
  get idAttribute() {
    return 'peerID';
  }

  url() {
    return app.getServerUrl('ob/chatconversation');
  }

  parse(response) {
    const processedMessage = processMessage(DOMPurify.sanitize((response.lastMessage)));

    return {
      ...response,
      lastMessage: processedMessage,
    };
  }
}
