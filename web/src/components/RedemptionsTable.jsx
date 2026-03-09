import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Form,
  Label,
  Popup,
  Pagination,
  Table,
} from 'semantic-ui-react';
import { Link, useNavigate } from 'react-router-dom';
import {
  API,
  copy,
  showError,
  showSuccess,
  showWarning,
  timestamp2string,
} from '../helpers';

import { ITEMS_PER_PAGE } from '../constants';
import { renderQuota } from '../helpers/render';

function renderTimestamp(timestamp) {
  return <>{timestamp2string(timestamp)}</>;
}

function renderStatus(status, t) {
  switch (status) {
    case 1:
      return (
        <Label basic color='green'>
          {t('redemption.status.unused')}
        </Label>
      );
    case 2:
      return (
        <Label basic color='red'>
          {t('redemption.status.disabled')}
        </Label>
      );
    case 3:
      return (
        <Label basic color='grey'>
          {t('redemption.status.used')}
        </Label>
      );
    default:
      return (
        <Label basic color='black'>
          {t('redemption.status.unknown')}
        </Label>
      );
  }
}

const RedemptionsTable = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [redemptions, setRedemptions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);

  const loadRedemptions = useCallback(async (startIdx) => {
    const res = await API.get(`/api/v1/admin/redemption/?p=${startIdx}`);
    const { success, message, data } = res.data;
    if (success) {
      if (startIdx === 0) {
        setRedemptions(data);
      } else {
        setRedemptions((prev) => [...prev, ...data]);
      }
    } else {
      showError(message);
    }
    setLoading(false);
  }, []);

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      if (activePage === Math.ceil(redemptions.length / ITEMS_PER_PAGE) + 1) {
        // In this case we have to load more data and then append them.
        await loadRedemptions(activePage - 1);
      }
      setActivePage(activePage);
    })();
  };

  useEffect(() => {
    loadRedemptions(0)
      .then()
      .catch((reason) => {
        showError(reason);
      });
  }, [loadRedemptions]);

  const manageRedemption = async (id, action, idx) => {
    let data = { id };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(`/api/v1/admin/redemption/${id}/`);
        break;
      case 'enable':
        data.status = 1;
        res = await API.put('/api/v1/admin/redemption/?status_only=true', data);
        break;
      case 'disable':
        data.status = 2;
        res = await API.put('/api/v1/admin/redemption/?status_only=true', data);
        break;
      default:
        return;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('token.messages.operation_success'));
      let redemption = res.data.data;
      let newRedemptions = [...redemptions];
      let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
      if (action === 'delete') {
        newRedemptions[realIdx].deleted = true;
      } else {
        newRedemptions[realIdx].status = redemption.status;
      }
      setRedemptions(newRedemptions);
    } else {
      showError(message);
    }
  };

  const searchRedemptions = async () => {
    if (searchKeyword === '') {
      // if keyword is blank, load files instead.
      await loadRedemptions(0);
      setActivePage(1);
      return;
    }
    setSearching(true);
    const res = await API.get(
      `/api/v1/admin/redemption/search?keyword=${searchKeyword}`
    );
    const { success, message, data } = res.data;
    if (success) {
      setRedemptions(data);
      setActivePage(1);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  const handleKeywordChange = async (e, { value }) => {
    setSearchKeyword(value.trim());
  };

  const sortRedemption = (key) => {
    if (redemptions.length === 0) return;
    setLoading(true);
    let sortedRedemptions = [...redemptions];
    sortedRedemptions.sort((a, b) => {
      if (!isNaN(a[key])) {
        // If the value is numeric, subtract to sort
        return a[key] - b[key];
      } else {
        // If the value is not numeric, sort as strings
        return ('' + a[key]).localeCompare(b[key]);
      }
    });
    if (sortedRedemptions[0].id === redemptions[0].id) {
      sortedRedemptions.reverse();
    }
    setRedemptions(sortedRedemptions);
    setLoading(false);
  };

  const refresh = async () => {
    setLoading(true);
    await loadRedemptions(0);
    setActivePage(1);
  };

  return (
    <>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '12px',
          flexWrap: 'wrap',
          marginBottom: '16px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Button
            className='router-page-button'
            as={Link}
            to='/redemption/add'
            loading={loading}
          >
            {t('redemption.buttons.add')}
          </Button>
          <Button className='router-page-button' onClick={refresh} loading={loading}>
            {t('redemption.buttons.refresh')}
          </Button>
        </div>
        <Form
          onSubmit={searchRedemptions}
          style={{ marginLeft: 'auto', width: 'min(360px, 100%)' }}
        >
          <Form.Input
            className='router-section-input'
            icon='search'
            fluid
            iconPosition='left'
            placeholder={t('redemption.search')}
            value={searchKeyword}
            loading={searching}
            onChange={handleKeywordChange}
          />
        </Form>
      </div>

      <Table basic={'very'} compact size='small' className='router-hover-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortRedemption('name');
              }}
            >
              {t('redemption.table.name')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortRedemption('status');
              }}
            >
              {t('redemption.table.status')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortRedemption('quota');
              }}
            >
              {t('redemption.table.quota')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortRedemption('created_time');
              }}
            >
              {t('redemption.table.created_time')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortRedemption('redeemed_time');
              }}
            >
              {t('redemption.table.redeemed_time')}
            </Table.HeaderCell>
            <Table.HeaderCell>{t('redemption.table.actions')}</Table.HeaderCell>
          </Table.Row>
        </Table.Header>

        <Table.Body>
          {redemptions
            .slice(
              (activePage - 1) * ITEMS_PER_PAGE,
              activePage * ITEMS_PER_PAGE
            )
            .map((redemption, idx) => {
              if (redemption.deleted) return <></>;
              return (
                <Table.Row
                  key={redemption.id}
                  style={{ cursor: 'pointer' }}
                  onClick={() => {
                    navigate(`/redemption/${redemption.id}`);
                  }}
                >
                  <Table.Cell>
                    {redemption.name ? redemption.name : t('redemption.table.no_name')}
                  </Table.Cell>
                  <Table.Cell>{renderStatus(redemption.status, t)}</Table.Cell>
                  <Table.Cell>{renderQuota(redemption.quota, t)}</Table.Cell>
                  <Table.Cell>
                    {renderTimestamp(redemption.created_time)}
                  </Table.Cell>
                  <Table.Cell>
                    {redemption.redeemed_time
                      ? renderTimestamp(redemption.redeemed_time)
                      : t('redemption.table.not_redeemed')}{' '}
                  </Table.Cell>
                  <Table.Cell
                    onClick={(e) => {
                      e.stopPropagation();
                    }}
                  >
                    <div>
                      <Button
                        className='router-inline-button'
                        positive
                        onClick={async () => {
                          if (await copy(redemption.code)) {
                            showSuccess(t('token.messages.copy_success'));
                          } else {
                            showWarning(t('token.messages.copy_failed'));
                            setSearchKeyword(redemption.code);
                          }
                        }}
                      >
                        {t('redemption.buttons.copy')}
                      </Button>
                      <Popup
                        trigger={
                          <Button className='router-inline-button' negative>
                            {t('redemption.buttons.delete')}
                          </Button>
                        }
                        on='click'
                        flowing
                        hoverable
                      >
                        <Button
                          className='router-inline-button'
                          negative
                          onClick={() => {
                            manageRedemption(redemption.id, 'delete', idx);
                          }}
                        >
                          {t('redemption.buttons.confirm_delete')}
                        </Button>
                      </Popup>
                      <Button
                        className='router-inline-button'
                        disabled={redemption.status === 3} // used
                        onClick={() => {
                          manageRedemption(
                            redemption.id,
                            redemption.status === 1 ? 'disable' : 'enable',
                            idx
                          );
                        }}
                      >
                        {redemption.status === 1
                          ? t('redemption.buttons.disable')
                          : t('redemption.buttons.enable')}
                      </Button>
                      <Button
                        className='router-inline-button'
                        as={Link}
                        to={'/redemption/edit/' + redemption.id}
                      >
                        {t('redemption.buttons.edit')}
                      </Button>
                    </div>
                  </Table.Cell>
                </Table.Row>
              );
            })}
        </Table.Body>

        <Table.Footer>
          <Table.Row>
            <Table.HeaderCell colSpan='6'>
              <Pagination
                floated='right'
                activePage={activePage}
                onPageChange={onPaginationChange}
                size='small'
                siblingRange={1}
                totalPages={
                  Math.ceil(redemptions.length / ITEMS_PER_PAGE) +
                  (redemptions.length % ITEMS_PER_PAGE === 0 ? 1 : 0)
                }
              />
            </Table.HeaderCell>
          </Table.Row>
        </Table.Footer>
      </Table>
    </>
  );
};

export default RedemptionsTable;
